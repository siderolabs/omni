// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

func TestInstallerExitError(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		reasonPhrase string
		code         int32
		grpcCode     codes.Code
		permanent    bool
	}{
		{
			name:         "invalid input is permanent",
			code:         constants.ExitInvalidInput,
			reasonPhrase: "machine configuration",
			grpcCode:     codes.InvalidArgument,
			permanent:    true,
		},
		{
			name:         "unsupported is permanent",
			code:         constants.ExitUnsupported,
			reasonPhrase: "does not support this operation",
			grpcCode:     codes.FailedPrecondition,
			permanent:    true,
		},
		{
			name:         "environment is retryable",
			code:         constants.ExitEnvironment,
			reasonPhrase: "pre-flight checks failed",
			grpcCode:     codes.FailedPrecondition,
			permanent:    false,
		},
		{
			name:         "dependency is retryable",
			code:         constants.ExitDependency,
			reasonPhrase: "external dependency failed",
			grpcCode:     codes.Unavailable,
			permanent:    false,
		},
		{
			name:         "io is retryable",
			code:         constants.ExitIO,
			reasonPhrase: "filesystem or I/O error",
			grpcCode:     codes.Internal,
			permanent:    false,
		},
		{
			name:         "install is retryable",
			code:         constants.ExitInstall,
			reasonPhrase: "writing Talos to the disk failed",
			grpcCode:     codes.Internal,
			permanent:    false,
		},
		{
			// Installers older than Talos 1.14 exit 1 for every failure, so this entry has to stay generic
			// and retryable.
			name:         "unknown error is generic and retryable",
			code:         constants.ExitUnknownError,
			reasonPhrase: "no specific reason",
			grpcCode:     codes.Internal,
			permanent:    false,
		},
		{
			name:         "an unrecognized code falls back to the generic entry",
			code:         42,
			reasonPhrase: "no specific reason",
			grpcCode:     codes.Internal,
			permanent:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &lifecycle.InstallerExitError{Operation: "upgrade", Code: tt.code}

			assert.Contains(t, err.Error(), tt.reasonPhrase)
			assert.Contains(t, err.Error(), "upgrade failed")
			assert.Contains(t, err.Error(), fmt.Sprintf("installer exit code %d", tt.code))

			assert.Equal(t, tt.grpcCode, status.Code(err))
			assert.Equal(t, tt.permanent, lifecycle.IsPermanentInstallerFailure(err))
		})
	}
}

// TestInstallerExitErrorSurvivesWrapping covers how the controllers actually see the error: the
// gRPC code has to survive WrapErr, and the permanence check has to survive plain error wrapping.
func TestInstallerExitErrorSurvivesWrapping(t *testing.T) {
	t.Parallel()

	err := &lifecycle.InstallerExitError{Operation: "installation", Code: constants.ExitInvalidInput}

	wrapped := lifecycle.WrapErr(err, "maintenance install failed")

	assert.Equal(t, codes.InvalidArgument, status.Code(wrapped))
	assert.Contains(t, wrapped.Error(), "maintenance install failed")
	assert.Contains(t, wrapped.Error(), "machine configuration")

	assert.True(t, lifecycle.IsPermanentInstallerFailure(fmt.Errorf("relaying progress: %w", err)))
}

func TestIsPermanentInstallerFailureOnOtherErrors(t *testing.T) {
	t.Parallel()

	assert.False(t, lifecycle.IsPermanentInstallerFailure(nil))
	assert.False(t, lifecycle.IsPermanentInstallerFailure(errors.New("connection refused")))
	assert.False(t, lifecycle.IsPermanentInstallerFailure(status.Error(codes.InvalidArgument, "not an installer failure")))
}
