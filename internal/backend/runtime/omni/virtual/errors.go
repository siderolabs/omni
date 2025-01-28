// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package virtual

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:errname
type eNotFound struct {
	error
}

func (eNotFound) NotFoundError() {}

func errNotFound(r resource.Pointer) error {
	return eNotFound{
		fmt.Errorf("resource %s doesn't exist", r),
	}
}

//nolint:errname
type eUnsupported struct {
	error
}

func (e eUnsupported) GRPCStatus() *status.Status {
	// if the wrapped error is already a status error, return it
	if sts, ok := status.FromError(e.error); ok {
		return sts
	}

	return status.New(codes.Unimplemented, e.Error())
}

func (eUnsupported) UnsupportedError() {}

func errUnsupported(err error) error {
	return eUnsupported{
		err,
	}
}
