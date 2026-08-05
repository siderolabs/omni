// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package supportbundle_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"github.com/siderolabs/go-talos-support/support/bundle/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/siderolabs/omni/client/pkg/supportbundle"
)

func TestValidateNoRecipientsLeft(t *testing.T) {
	t.Parallel()

	err := supportbundle.EncryptionOptions{NoDefaultRecipients: true}.Validate()
	require.Error(t, err)

	_, _, err = supportbundle.Encrypt(io.Discard, supportbundle.EncryptionOptions{NoDefaultRecipients: true})
	require.Error(t, err)
}

func TestEncryptDefaultRecipients(t *testing.T) {
	t.Parallel()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	extra := identity.Recipient().String()

	writer, recipients, err := supportbundle.Encrypt(io.Discard, supportbundle.EncryptionOptions{
		Recipients: []string{extra},
	})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	defaults, err := encryption.DefaultRecipients()
	require.NoError(t, err)
	require.NotEmpty(t, defaults, "go-talos-support ships no default recipients")

	// the Sidero Labs recipients are reported alongside the extra one, so the user can see everyone
	// who will be able to open the bundle.
	assert.Len(t, recipients, len(defaults)+1)
	assert.Equal(t, extra, recipients[len(recipients)-1])
	assert.Contains(t, recipients, defaults[0].String())
}

func TestEncryptOnlyGivenRecipients(t *testing.T) {
	t.Parallel()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	plaintext := []byte("support bundle contents")

	var encrypted bytes.Buffer

	writer, recipients, err := supportbundle.Encrypt(&encrypted, supportbundle.EncryptionOptions{
		Recipients:          []string{identity.Recipient().String()},
		NoDefaultRecipients: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{identity.Recipient().String()}, recipients)

	_, err = writer.Write(plaintext)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	assert.True(t, bytes.HasPrefix(encrypted.Bytes(), []byte("age-encryption.org/v1")))

	// dropping the defaults must really drop them: the generated identity alone opens the bundle.
	decrypted, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), identity)
	require.NoError(t, err)

	roundTripped, err := io.ReadAll(decrypted)
	require.NoError(t, err)

	assert.Equal(t, plaintext, roundTripped)
}

// TestEncryptSSHRecipient covers the recipient format users actually paste, an OpenSSH public key
// line, added on top of the built-in Sidero Labs defaults.
func TestEncryptSSHRecipient(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	require.NoError(t, err)

	identity, err := agessh.NewEd25519Identity(privateKey)
	require.NoError(t, err)

	recipient := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey)))

	plaintext := []byte("support bundle contents")

	var encrypted bytes.Buffer

	writer, recipients, err := supportbundle.Encrypt(&encrypted, supportbundle.EncryptionOptions{
		Recipients: []string{recipient},
	})
	require.NoError(t, err)

	assert.Contains(t, recipients, recipient)

	_, err = writer.Write(plaintext)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// the added SSH key opens the bundle even though the Sidero Labs defaults are also recipients.
	decrypted, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), identity)
	require.NoError(t, err)

	roundTripped, err := io.ReadAll(decrypted)
	require.NoError(t, err)

	assert.Equal(t, plaintext, roundTripped)
}

func TestEncryptMalformedRecipient(t *testing.T) {
	t.Parallel()

	_, _, err := supportbundle.Encrypt(io.Discard, supportbundle.EncryptionOptions{
		Recipients:          []string{"not-a-public-key"},
		NoDefaultRecipients: true,
	})
	require.Error(t, err)
}
