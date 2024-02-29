// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logformat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// NewDmesgOutput returns a new DmesgOutput.
func NewDmesgOutput(rdr io.Reader) *DmesgOutput {
	return &DmesgOutput{
		decoder: json.NewDecoder(rdr),
	}
}

// DmesgOutput is used to print logs in dmesg format.
type DmesgOutput struct {
	decoder *json.Decoder
}

// Run parses logs from passed reader and prints them in dmesg format until the context is canceled
// or the reader has returned an error.
func (o DmesgOutput) Run(ctx context.Context) error {
	var msg talosLogMessage

	for {
		if ctx.Err() != nil {
			return nil //nolint:nilerr
		}

		err := o.decoder.Decode(&msg)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		secondsPart := msg.Clock / 1000000
		microSecondsPart := msg.Clock % 1000000
		message := trimNewLine(msg.Message)

		if strings.IndexByte(message, '\n') == -1 {
			fmt.Printf("[%5d.%06d] %s: %s\n", secondsPart, microSecondsPart, msg.Facility, message)
		} else {
			for i, line := range strings.Split(message, "\n") {
				if i == 0 {
					fmt.Printf("[%5d.%06d] %s: %s\n", secondsPart, microSecondsPart, msg.Facility, line)
				} else {
					fmt.Printf("[%5d.%06d] %s\n", secondsPart, microSecondsPart, line)
				}
			}
		}
	}
}

func trimNewLine(message string) string {
	if len(message) > 0 && message[len(message)-1] == '\n' {
		message = message[:len(message)-1]
	}

	return message
}
