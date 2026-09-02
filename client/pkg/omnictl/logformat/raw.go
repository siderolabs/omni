// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logformat

import (
	"io"

	"github.com/siderolabs/omni/client/internal/safeout"
)

// NewRawOutput runs the raw log format.
func NewRawOutput(rdr io.Reader) *RawOutput {
	return &RawOutput{
		rdr: rdr,
	}
}

// RawOutput simply copies the input rdr to the standard output.
type RawOutput struct {
	rdr io.Reader
}

// Run copies the input rdr to the standard output until the rdr has returned an error.
func (o RawOutput) Run() error {
	_, err := io.Copy(safeout.Stdout(), o.rdr)

	return err
}
