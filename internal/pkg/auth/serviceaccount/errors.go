// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package serviceaccount

import "github.com/cosi-project/runtime/pkg/resource"

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
