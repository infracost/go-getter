package getter

import (
	"net/url"
	"testing"
)

func TestArchiveURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		ref     string
		want    string
		wantErr bool
	}{
		{
			name: "github with .git suffix",
			url:  "https://github.com/hashicorp/terraform.git",
			ref:  "abc1234",
			want: "https://github.com/hashicorp/terraform/archive/abc1234.tar.gz",
		},
		{
			name: "github without .git suffix",
			url:  "https://github.com/hashicorp/terraform",
			ref:  "abc1234",
			want: "https://github.com/hashicorp/terraform/archive/abc1234.tar.gz",
		},
		{
			name: "gitlab",
			url:  "https://gitlab.com/myorg/myrepo.git",
			ref:  "def5678",
			want: "https://gitlab.com/myorg/myrepo/-/archive/def5678/myrepo-def5678.tar.gz",
		},
		{
			name: "bitbucket",
			url:  "https://bitbucket.org/myorg/myrepo.git",
			ref:  "aaa1111",
			want: "https://bitbucket.org/myorg/myrepo/get/aaa1111.tar.gz",
		},
		{
			name: "github via ssh",
			url:  "ssh://git@github.com/hashicorp/terraform.git",
			ref:  "abc1234",
			want: "https://github.com/hashicorp/terraform/archive/abc1234.tar.gz",
		},
		{
			name: "github via http",
			url:  "http://github.com/hashicorp/terraform.git",
			ref:  "abc1234",
			want: "https://github.com/hashicorp/terraform/archive/abc1234.tar.gz",
		},
		{
			name: "github via git scheme",
			url:  "git://github.com/hashicorp/terraform.git",
			ref:  "abc1234",
			want: "https://github.com/hashicorp/terraform/archive/abc1234.tar.gz",
		},
		{
			name: "gitlab via ssh",
			url:  "ssh://git@gitlab.com/myorg/myrepo.git",
			ref:  "def5678",
			want: "https://gitlab.com/myorg/myrepo/-/archive/def5678/myrepo-def5678.tar.gz",
		},
		{
			name: "bitbucket via ssh",
			url:  "ssh://git@bitbucket.org/myorg/myrepo.git",
			ref:  "aaa1111",
			want: "https://bitbucket.org/myorg/myrepo/get/aaa1111.tar.gz",
		},
		{
			name:    "unsupported host",
			url:     "https://example.com/owner/repo.git",
			ref:     "abc1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", tt.url, err)
			}

			got, err := archiveURL(u, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Fatalf("archiveURL() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("archiveURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "with .git suffix",
			path:      "/hashicorp/terraform.git",
			wantOwner: "hashicorp",
			wantRepo:  "terraform",
		},
		{
			name:      "without .git suffix",
			path:      "/hashicorp/terraform",
			wantOwner: "hashicorp",
			wantRepo:  "terraform",
		},
		{
			name:      "extra path segments are ignored",
			path:      "/org/repo/extra/path",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:    "single segment is invalid",
			path:    "/onlyone",
			wantErr: true,
		},
		{
			name:    "empty path is invalid",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseOwnerRepo(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOwnerRepo(%q): err=%v, wantErr=%v", tt.path, err, tt.wantErr)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("parseOwnerRepo(%q) = (%q, %q), want (%q, %q)", tt.path, owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestTokenFromEnv(t *testing.T) {
	tests := []struct {
		name string
		host string
		vars map[string]string
		want string
	}{
		{
			name: "GH_TOKEN",
			host: "github.com",
			vars: map[string]string{"GH_TOKEN": "ghtoken123"},
			want: "ghtoken123",
		},
		{
			name: "GITHUB_TOKEN",
			host: "github.com",
			vars: map[string]string{"GITHUB_TOKEN": "ghtoken456"},
			want: "ghtoken456",
		},
		{
			name: "GH_TOKEN takes precedence over GITHUB_TOKEN",
			host: "github.com",
			vars: map[string]string{"GH_TOKEN": "primary", "GITHUB_TOKEN": "secondary"},
			want: "primary",
		},
		{
			name: "GITLAB_TOKEN",
			host: "gitlab.com",
			vars: map[string]string{"GITLAB_TOKEN": "gltoken123"},
			want: "gltoken123",
		},
		{
			name: "GL_TOKEN",
			host: "gitlab.com",
			vars: map[string]string{"GL_TOKEN": "gltoken456"},
			want: "gltoken456",
		},
		{
			name: "GITLAB_TOKEN takes precedence over GL_TOKEN",
			host: "gitlab.com",
			vars: map[string]string{"GITLAB_TOKEN": "primary", "GL_TOKEN": "secondary"},
			want: "primary",
		},
		{
			name: "BITBUCKET_TOKEN",
			host: "bitbucket.org",
			vars: map[string]string{"BITBUCKET_TOKEN": "bbtoken"},
			want: "bbtoken",
		},
		{
			name: "unsupported host",
			host: "example.com",
			vars: map[string]string{},
			want: "",
		},
		{
			name: "github with port",
			host: "github.com:443",
			vars: map[string]string{"GH_TOKEN": "tok"},
			want: "tok",
		},
	}

	// Clear all relevant env vars before each subtest.
	envVars := []string{"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "GL_TOKEN", "BITBUCKET_TOKEN"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range envVars {
				t.Setenv(k, "")
			}
			for k, v := range tt.vars {
				t.Setenv(k, v)
			}

			got := tokenFromEnv(tt.host)
			if got != tt.want {
				t.Errorf("tokenFromEnv(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}