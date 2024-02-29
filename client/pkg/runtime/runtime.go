// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime contains public interfaces used in Omni frontend runtime.
package runtime

// Matcher is implemented by all types which can traverse themselves in search for specific string.
type Matcher interface {
	Match(string) bool
}

// Fielder is implemented by all types which can traverse themselves in search for specific field.
type Fielder interface {
	Field(string) (string, bool)
}

// ListItem is a wrapper for the list item.
type ListItem interface {
	Matcher
	ID() string
	Namespace() string
	Unwrap() any
	Field(name string) (string, bool)
}
