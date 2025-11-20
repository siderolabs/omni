// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package errors implements various COSI errors used in the virtual state.
package errors

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:errname
type NotFound struct {
	error
}

func (NotFound) NotFoundError() {}

// ErrNotFound creates new not found error.
func ErrNotFound(r resource.Pointer) error {
	return NotFound{
		fmt.Errorf("resource %s doesn't exist", r),
	}
}

//nolint:errname
type Unsupported struct {
	error
}

func (e Unsupported) GRPCStatus() *status.Status {
	// if the wrapped error is already a status error, return it
	if sts, ok := status.FromError(e.error); ok {
		return sts
	}

	return status.New(codes.Unimplemented, e.Error())
}

func (Unsupported) UnsupportedError() {}

// ErrUnsupported creates a new unsupported error.
func ErrUnsupported(err error) error {
	return Unsupported{
		err,
	}
}
