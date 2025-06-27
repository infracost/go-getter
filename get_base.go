// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package getter

// getter is our base getter; it regroups
// fields all getters have in common.
type getter struct {
	client *Client
}

func (g *getter) SetClient(c *Client) { g.client = c }
