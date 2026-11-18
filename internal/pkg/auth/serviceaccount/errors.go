// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package serviceaccount

import (
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
)

// ErrInvalid is the error of a request which cannot be fulfilled as it is, e.g., a role which does not fit the name of the service account.
var ErrInvalid = errors.New("invalid service account request")

//nolint:errname
type eNotFound struct {
	error
}

func (*eNotFound) NotFoundError() {}

//nolint:errname
type eConflict struct {
	error

	res resource.Pointer
}

func (*eConflict) ConflictError() {}

func (e *eConflict) GetResource() resource.Pointer {
	return e.res
}
