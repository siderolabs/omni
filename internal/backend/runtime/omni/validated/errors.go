// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errPrefix = "failed to validate: "

// ErrValidation should be implemented by validation errors.
type ErrValidation interface {
	ValidationError()
}

// IsValidationError checks if err is validation error.
func IsValidationError(err error) bool {
	var i ErrValidation

	if errors.As(err, &i) {
		return true
	}

	sts, ok := status.FromError(err)
	if !ok {
		return false
	}

	return sts.Code() == codes.InvalidArgument && strings.HasPrefix(sts.Message(), errPrefix)
}

type eValidation struct {
	error
}

func (e eValidation) ValidationError() {
}

func (e eValidation) GRPCStatus() *status.Status {
	// if the wrapped error is already a status error, return it
	if sts, ok := status.FromError(e.error); ok {
		return sts
	}

	return status.New(codes.InvalidArgument, e.Error())
}

// ValidationError generates error compatible with validated.ErrValidation.
func ValidationError(err error) error {
	return eValidation{
		fmt.Errorf("%s%w", errPrefix, err),
	}
}
