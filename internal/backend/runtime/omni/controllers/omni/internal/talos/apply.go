// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ApplyMayHaveLanded reports whether a failed apply RPC can still have reached the machine: the request went out, but the answer did not come back.
func ApplyMayHaveLanded(err error) bool {
	code := status.Code(err)

	return code == codes.DeadlineExceeded || code == codes.Canceled || errors.Is(err, context.DeadlineExceeded)
}

// CheckBeforeApply checks the machine before a config apply and returns its boot ID.
//
// A confirm (expectBootID set) fails when the machine no longer reports that boot ID, as it has dropped the try config with its reboot,
// and when the tried config is not active on it, as it never got the try or has rolled it back. A try returns the boot ID it starts on, and
// fails when it cannot read one.
func CheckBeforeApply(
	ctx context.Context,
	readBootID func(context.Context) (string, error),
	triedConfigIsActive func(context.Context) (bool, error),
	mode machine.ApplyConfigurationRequest_Mode,
	expectBootID string,
) (string, error) {
	if expectBootID == "" && mode != machine.ApplyConfigurationRequest_TRY {
		return "", nil
	}

	bootID, err := readBootID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read the boot ID: %w", err)
	}

	if bootID == "" {
		return "", errors.New("the machine reported an empty boot ID")
	}

	if expectBootID != "" && bootID != expectBootID {
		return "", fmt.Errorf("machine rebooted (boot ID %q became %q)", expectBootID, bootID)
	}

	if expectBootID != "" {
		active, err := triedConfigIsActive(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to check the active config: %w", err)
		}

		if !active {
			return "", errors.New("the machine is not running the tried config, it never got it or rolled it back")
		}
	}

	return bootID, nil
}
