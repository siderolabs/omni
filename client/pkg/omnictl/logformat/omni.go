// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logformat contains different implementations of log formatting.
package logformat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"go.uber.org/zap"
)

// NewOmniOutput returns a new OmniOutput.
func NewOmniOutput(rdr io.Reader) *OmniOutput {
	decoder := json.NewDecoder(rdr)

	return &OmniOutput{decoder: decoder}
}

// OmniOutput is a log output which parses logs from Talos machine and prints them in zap'like format.
type OmniOutput struct {
	decoder *json.Decoder
}

// Run prints logs from passed reader until the context is canceled or the reader has returned an error.
func (o *OmniOutput) Run(ctx context.Context) error {
	var msg talosLogMessage

	var logger zap.Logger

	logger.WithOptions(zap.AddStacktrace(zap.FatalLevel))

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

		level, err := zap.ParseAtomicLevel(msg.TalosLevel)
		if err != nil {
			level = zap.NewAtomicLevel()
		}

		logMsg := logger.Check(level.Level(), msg.Message)
		if logMsg != nil {
			logMsg.Write(
				zap.Int("clock", msg.Clock),
				zap.String("facility", msg.Facility),
				zap.String("priority", msg.Priority),
				zap.Int("seq", msg.Seq),
				zap.Time("talos-time", msg.TalosTime),
			)
		}
	}
}

type talosLogMessage struct {
	TalosTime  time.Time `json:"talos-time"`
	Facility   string    `json:"facility"`
	Message    string    `json:"msg"`
	Priority   string    `json:"priority"`
	TalosLevel string    `json:"talos-level"`
	Clock      int       `json:"clock"`
	Seq        int       `json:"seq"`
}
