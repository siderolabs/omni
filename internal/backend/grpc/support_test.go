// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"filippo.io/age"
	"github.com/siderolabs/go-talos-support/support"
	"github.com/siderolabs/go-talos-support/support/bundle"
	"github.com/siderolabs/go-talos-support/support/collectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
)

// TestEncryptedOutputInvalidRequests checks that a request the client should never have sent is
// rejected as an invalid argument rather than producing a bundle nobody can open.
func TestEncryptedOutputInvalidRequests(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		req  *management.GetSupportBundleRequest
		name string
	}{
		{
			name: "recipients without encryption",
			req: &management.GetSupportBundleRequest{
				EncryptionRecipients: []string{"age1qqqqq"},
			},
		},
		{
			name: "no default recipients without encryption",
			req: &management.GetSupportBundleRequest{
				EncryptionNoDefaultRecipients: true,
			},
		},
		{
			name: "no recipients left to encrypt to",
			req: &management.GetSupportBundleRequest{
				Encrypt:                       true,
				EncryptionNoDefaultRecipients: true,
			},
		},
		{
			name: "malformed recipient key",
			req: &management.GetSupportBundleRequest{
				Encrypt:              true,
				EncryptionRecipients: []string{"not-a-public-key"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stream := &supportBundleStreamMock{ctx: t.Context()}

			_, _, err := grpcomni.EncryptedOutput(tt.req, io.Discard, stream)

			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Empty(t, stream.responses, "nothing should be streamed for a rejected request")
		})
	}
}

// supportBundleStreamMock records what the server streams back to the client.
type supportBundleStreamMock struct {
	grpc.ServerStream

	ctx       context.Context //nolint:containedctx
	responses []*management.GetSupportBundleResponse
}

func (s *supportBundleStreamMock) Send(resp *management.GetSupportBundleResponse) error {
	s.responses = append(s.responses, resp)

	return nil
}

func (s *supportBundleStreamMock) Context() context.Context {
	return s.ctx
}

// TestEncryptedOutputRoundTrip checks that a bundle built through the encrypted writer survives a
// decrypt and unzip.
func TestEncryptedOutputRoundTrip(t *testing.T) {
	t.Parallel()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	stream := &supportBundleStreamMock{ctx: t.Context()}

	var encrypted bytes.Buffer

	archiveOutput, flush, err := grpcomni.EncryptedOutput(&management.GetSupportBundleRequest{
		Encrypt:                       true,
		EncryptionNoDefaultRecipients: true,
		EncryptionRecipients:          []string{identity.Recipient().String()},
	}, &encrypted, stream)
	require.NoError(t, err)

	// the server reports the recipients before collection starts, so the client can show them while it waits.
	require.Len(t, stream.responses, 1)
	assert.Equal(t, []string{identity.Recipient().String()}, stream.responses[0].EncryptionRecipients)
	assert.Nil(t, stream.responses[0].BundleData)

	contents := []byte("dmesg contents")

	options := bundle.NewOptions(
		bundle.WithArchiveOutput(archiveOutput),
		bundle.WithLogOutput(io.Discard),
	)

	require.NoError(t, support.CreateSupportBundle(
		t.Context(), options,
		collectors.NewCollector("node/dmesg.log", func() ([]byte, error) { return contents, nil }),
	))

	require.NoError(t, flush())

	decrypted, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), identity)
	require.NoError(t, err)

	raw, err := io.ReadAll(decrypted)
	require.NoError(t, err)

	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)

	f, err := archive.Open("node/dmesg.log")
	require.NoError(t, err)

	defer f.Close() //nolint:errcheck

	collected, err := io.ReadAll(f)
	require.NoError(t, err)

	assert.Equal(t, contents, collected)
}

// TestEncryptedOutputWithoutEncryption checks that the unencrypted path stays a plain zip and says
// nothing about recipients.
func TestEncryptedOutputWithoutEncryption(t *testing.T) {
	t.Parallel()

	stream := &supportBundleStreamMock{ctx: t.Context()}

	var out bytes.Buffer

	archiveOutput, flush, err := grpcomni.EncryptedOutput(&management.GetSupportBundleRequest{}, &out, stream)
	require.NoError(t, err)

	assert.Empty(t, stream.responses)

	options := bundle.NewOptions(
		bundle.WithArchiveOutput(archiveOutput),
		bundle.WithLogOutput(io.Discard),
	)

	require.NoError(t, support.CreateSupportBundle(
		t.Context(), options,
		collectors.NewCollector("node/dmesg.log", func() ([]byte, error) { return []byte("dmesg contents"), nil }),
	))

	require.NoError(t, flush())

	_, err = zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	require.NoError(t, err)
}
