// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package supportbundle holds the support bundle helpers shared by the Omni server and omnictl.
package supportbundle

import (
	"errors"
	"io"
	"slices"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-talos-support/support/bundle/encryption"
)

// EncryptionOptions describes who a support bundle should be encrypted to.
//
// The semantics mirror the `talosctl support` flags, so both CLIs behave the same way.
type EncryptionOptions struct {
	// Recipients are age recipients: SSH public keys (ssh-rsa, ssh-ed25519) or "age1..." recipients.
	Recipients []string

	// NoDefaultRecipients drops the built-in Sidero Labs recipients, leaving only Recipients.
	NoDefaultRecipients bool
}

// Validate reports whether the options can produce a bundle anyone is able to open.
func (o EncryptionOptions) Validate() error {
	if o.NoDefaultRecipients && len(o.Recipients) == 0 {
		return errors.New("no recipients to encrypt to: default recipients are disabled but no recipients were provided")
	}

	return nil
}

// Encrypt wraps dst in an age layer and reports the recipients able to decrypt whatever is written
// to it.
//
// The returned writer must be closed to flush the age layer. dst is left open.
func Encrypt(dst io.Writer, o EncryptionOptions) (io.WriteCloser, []string, error) {
	if err := o.Validate(); err != nil {
		return nil, nil, err
	}

	if o.NoDefaultRecipients {
		writer, err := encryption.Encrypt(dst, encryption.WithRecipients(o.Recipients...))
		if err != nil {
			return nil, nil, err
		}

		return writer, slices.Clone(o.Recipients), nil
	}

	defaults, err := encryption.DefaultRecipients()
	if err != nil {
		return nil, nil, err
	}

	recipients := xslices.Map(defaults, encryption.Recipient.String)

	var opts []encryption.Option

	if len(o.Recipients) > 0 {
		opts = append(opts, encryption.WithAdditionalRecipients(o.Recipients...))
		recipients = append(recipients, o.Recipients...)
	}

	writer, err := encryption.Encrypt(dst, opts...)
	if err != nil {
		return nil, nil, err
	}

	return writer, recipients, nil
}
