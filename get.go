// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// getter is a package for downloading files or directories from a variety of
// protocols.
//
// getter is unique in its ability to download both directories and files.
// It also detects certain source strings to be protocol-specific URLs. For
// example, "github.com/hashicorp/go-getter" would turn into a Git URL and
// use the Git protocol.
//
// Protocols and detectors are extensible.
//
// To get started, see Client.
package getter

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
)

// Getter defines the interface that schemes must implement to download
// things.
type Getter interface {
	// Get downloads the given URL into the given directory. This always
	// assumes that we're updating and gets the latest version that it can.
	//
	// The directory may already exist (if we're updating). If it is in a
	// format that isn't understood, an error should be returned. Get shouldn't
	// simply nuke the directory.
	Get(context.Context, string, *url.URL) error

	// GetFile downloads the give URL into the given path. The URL must
	// reference a single file. If possible, the Getter should check if
	// the remote end contains the same file and no-op this operation.
	GetFile(context.Context, string, *url.URL) error

	// ClientMode returns the mode based on the given URL. This is used to
	// allow clients to let the getters decide which mode to use.
	ClientMode(context.Context, *url.URL) (ClientMode, error)

	// SetClient allows a getter to know it's client
	// in order to access client's Get functions or
	// progress tracking.
	SetClient(*Client)
}

// Getters is the mapping of scheme to the Getter implementation that will
// be used to get a dependency.
var Getters map[string]Getter

// forcedRegexp is the regular expression that finds forced getters. This
// syntax is schema::url, example: git::https://foo.com
var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

// httpClient is the default client to be used by HttpGetters.
var httpClient = cleanhttp.DefaultClient()

func init() {
	httpGetter := &HttpGetter{
		Netrc: true,
	}

	Getters = map[string]Getter{
		"file":  new(FileGetter),
		"git":   new(GitGetter),
		"gcs":   new(GCSGetter),
		"hg":    new(HgGetter),
		"s3":    new(S3Getter),
		"http":  httpGetter,
		"https": httpGetter,
	}
}

// Get downloads the directory specified by src into the folder specified by
// dst. If dst already exists, Get will attempt to update it.
//
// src is a URL, whereas dst is always just a file path to a folder. This
// folder doesn't need to exist. It will be created if it doesn't exist.
func Get(ctx context.Context, dst, src string, opts ...ClientOption) error {
	return (&Client{
		Src:     src,
		Dst:     dst,
		Dir:     true,
		Options: opts,
	}).Get(ctx)
}

// GetAny downloads a URL into the given destination. Unlike Get or
// GetFile, both directories and files are supported.
//
// dst must be a directory. If src is a file, it will be downloaded
// into dst with the basename of the URL. If src is a directory or
// archive, it will be unpacked directly into dst.
func GetAny(ctx context.Context, dst, src string, opts ...ClientOption) error {
	return (&Client{
		Src:     src,
		Dst:     dst,
		Mode:    ClientModeAny,
		Options: opts,
	}).Get(ctx)
}

// GetFile downloads the file specified by src into the path specified by
// dst.
func GetFile(ctx context.Context, dst, src string, opts ...ClientOption) error {
	return (&Client{
		Src:     src,
		Dst:     dst,
		Dir:     false,
		Options: opts,
	}).Get(ctx)
}

// getRunCommand is a helper that will run a command and capture the output
// in the case an error happens.
func getRunCommand(cmd *exec.Cmd) error {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0
		return fmt.Errorf(
			"%s exited with %s: %s",
			cmd.Path,
			exiterr.ProcessState.String(),
			buf.String())
	}

	return fmt.Errorf("error running %s: %s", cmd.Path, buf.String())
}

// getForcedGetter takes a source and returns the tuple of the forced
// getter and the raw URL (without the force syntax).
func getForcedGetter(src string) (string, string) {
	var forced string
	if ms := forcedRegexp.FindStringSubmatch(src); ms != nil {
		forced = ms[1]
		src = ms[2]
	}

	return forced, src
}
