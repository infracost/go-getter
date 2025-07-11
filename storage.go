// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package getter

import "context"

// Storage is an interface that knows how to lookup downloaded directories
// as well as download and update directories from their sources into the
// proper location.
type Storage interface {
	// Dir returns the directory on local disk where the directory source
	// can be loaded from.
	Dir(string) (string, bool, error)

	// Get will download and optionally update the given directory.
	Get(context.Context, string, string, bool) error
}
