package getter

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	safetemp "github.com/hashicorp/go-safetemp"
)

// fetchArchive downloads a tarball archive of the given commit from the
// hosting platform's HTTP API and extracts it to dst. This is used as a
// fallback when git-fetch cannot retrieve a commit (e.g. orphaned commits
// that are unreachable from any ref).
//
// Authentication is resolved from URL userinfo, netrc, or environment
// variables (GH_TOKEN, GITLAB_TOKEN, etc.). SSH keys cannot be used here
// since this is an HTTP download — if the user only has SSH credentials
// configured, this fallback will fail for private repositories.
//
// If subdir is non-empty, only files under that subdirectory are placed in
// dst. The resulting directory is NOT a git repository.
func fetchArchive(ctx context.Context, dst string, u *url.URL, ref string, subdir string) error {
	aURL, err := archiveURL(u, ref)
	if err != nil {
		return err
	}

	// Parse the archive URL so we can attach credentials.
	archiveParsed, err := url.Parse(aURL)
	if err != nil {
		return err
	}

	// Carry over credentials from the original git URL if present,
	// otherwise fall back to the user's netrc file. Skip the common SSH
	// placeholder user "git" since it isn't a real credential.
	if u.User != nil && u.User.Username() != "" && u.User.Username() != "git" {
		archiveParsed.User = u.User
	} else if err := addAuthFromNetrc(archiveParsed); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveParsed.String(), nil)
	if err != nil {
		return err
	}

	if archiveParsed.User != nil {
		password, _ := archiveParsed.User.Password()
		req.SetBasicAuth(archiveParsed.User.Username(), password)
	} else if token := tokenFromEnv(archiveParsed.Host); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download archive from %s: %w", aURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download archive from %s: HTTP %d", aURL, resp.StatusCode)
	}

	// The tarball contains a single top-level directory (e.g. "repo-sha/").
	// We extract into a temp directory first, then move the contents into dst.
	td, tdcloser, err := safetemp.Dir("", "go-getter-archive")
	if err != nil {
		return err
	}
	defer func() { _ = tdcloser.Close() }()

	gzipR, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to decompress archive: %w", err)
	}
	defer func() { _ = gzipR.Close() }()

	if err := untar(gzipR, td, aURL, true, 0, 0, 0); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Find the single top-level directory that the archive extracted into.
	entries, err := os.ReadDir(td)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("archive contained no files")
	}

	srcDir := filepath.Join(td, entries[0].Name())
	if subdir != "" {
		srcDir = filepath.Join(srcDir, subdir)
	}

	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("path %q not found in archive", subdir)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return copyDir(ctx, dst, srcDir, false, false, 0)
}

// archiveURL constructs a tarball download URL for the given ref based on the
// hosting platform detected from u's hostname.
func archiveURL(u *url.URL, ref string) (string, error) {
	owner, repo, err := parseOwnerRepo(u.Path)
	if err != nil {
		return "", err
	}

	host := strings.ToLower(u.Host)
	// Strip port if present (e.g. "github.com:443" → "github.com").
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", owner, repo, ref), nil
	case host == "gitlab.com" || strings.HasSuffix(host, ".gitlab.com"):
		return fmt.Sprintf("https://gitlab.com/%s/%s/-/archive/%s/%s-%s.tar.gz", owner, repo, ref, repo, ref), nil
	case host == "bitbucket.org" || strings.HasSuffix(host, ".bitbucket.org"):
		return fmt.Sprintf("https://bitbucket.org/%s/%s/get/%s.tar.gz", owner, repo, ref), nil
	default:
		return "", fmt.Errorf("unsupported git hosting platform %q for archive fallback", u.Host)
	}
}

// tokenFromEnv returns an API token from well-known environment variables
// for the given host. It returns an empty string if no token is found.
func tokenFromEnv(host string) string {
	host = strings.ToLower(host)
	// Strip port if present.
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		// GH_TOKEN is the newer GitHub CLI convention; GITHUB_TOKEN is the
		// widely-used CI/Actions variable.
		if t := os.Getenv("GH_TOKEN"); t != "" {
			return t
		}
		return os.Getenv("GITHUB_TOKEN")
	case host == "gitlab.com" || strings.HasSuffix(host, ".gitlab.com"):
		if t := os.Getenv("GITLAB_TOKEN"); t != "" {
			return t
		}
		return os.Getenv("GL_TOKEN")
	case host == "bitbucket.org" || strings.HasSuffix(host, ".bitbucket.org"):
		return os.Getenv("BITBUCKET_TOKEN")
	default:
		return ""
	}
}

// parseOwnerRepo extracts the owner and repository name from a URL path
// like "/owner/repo.git" or "/owner/repo".
func parseOwnerRepo(rawPath string) (owner, repo string, err error) {
	path := strings.TrimPrefix(rawPath, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from path %q", rawPath)
	}
	return parts[0], parts[1], nil
}