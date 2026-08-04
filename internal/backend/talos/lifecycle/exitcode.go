// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InstallerExitError reports a non-zero exit from the installer container Talos ran for an install or upgrade.
type InstallerExitError struct {
	// Operation labels the failed operation for the operator, e.g. "installation" or "upgrade".
	Operation string
	Code      int32
}

// Error implements the error interface.
func (e *InstallerExitError) Error() string {
	return fmt.Sprintf("%s failed: %s (installer exit code %d)", e.Operation, classifyInstallerExit(e.Code).reason, e.Code)
}

// GRPCStatus carries the mapped code through WrapErr and the management API, both of which read it
// back with status.FromError.
func (e *InstallerExitError) GRPCStatus() *status.Status {
	return status.New(classifyInstallerExit(e.Code).grpcCode, e.Error())
}

// Permanent reports whether rerunning the same installer over the same inputs can only fail the same way.
func (e *InstallerExitError) Permanent() bool {
	return classifyInstallerExit(e.Code).permanent
}

// IsPermanentInstallerFailure reports whether err is an installer failure that no retry can fix.
func IsPermanentInstallerFailure(err error) bool {
	var exitErr *InstallerExitError

	return errors.As(err, &exitErr) && exitErr.Permanent()
}

// installerExitInfo is Omni's presentation of one installer exit code.
type installerExitInfo struct {
	reason    string
	grpcCode  codes.Code
	permanent bool
}

// classifyInstallerExit maps a Talos installer exit code onto an operator-facing reason and a gRPC code.
// Installers older than Talos 1.14 exit 1 for every failure, which lands on the generic entry below and stays retryable.
func classifyInstallerExit(code int32) installerExitInfo {
	switch code {
	case constants.ExitInvalidInput:
		return installerExitInfo{
			reason:    "the installer rejected its input, the machine configuration or the install options are not valid for this Talos version",
			grpcCode:  codes.InvalidArgument,
			permanent: true,
		}
	case constants.ExitUnsupported:
		return installerExitInfo{
			reason:    "the installer does not support this operation for the selected version, platform or feature set",
			grpcCode:  codes.FailedPrecondition,
			permanent: true,
		}
	case constants.ExitEnvironment:
		return installerExitInfo{
			reason:    "the machine does not meet the installer's prerequisites, or its pre-flight checks failed",
			grpcCode:  codes.FailedPrecondition,
			permanent: false,
		}
	case constants.ExitDependency:
		return installerExitInfo{
			reason:    "an external dependency failed, such as the installer image pull, signature verification or post-processing",
			grpcCode:  codes.Unavailable,
			permanent: false,
		}
	case constants.ExitIO:
		return installerExitInfo{
			reason:    "the installer hit a filesystem or I/O error on the machine",
			grpcCode:  codes.Internal,
			permanent: false,
		}
	case constants.ExitInstall:
		return installerExitInfo{
			reason:    "writing Talos to the disk failed",
			grpcCode:  codes.Internal,
			permanent: false,
		}
	default:
		return installerExitInfo{
			reason:    "the installer reported no specific reason, check the machine logs",
			grpcCode:  codes.Internal,
			permanent: false,
		}
	}
}
