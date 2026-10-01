// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated_test

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// TestValidationErrorKeepsOnlyTheRefusal checks that a refused request is answered by that refusal, and that what the
// other validations reported does not travel back with it.
func TestValidationErrorKeepsOnlyTheRefusal(t *testing.T) {
	refusal := status.Error(codes.PermissionDenied, `insufficient role: "None"`)

	for _, test := range []struct {
		name string
		errs []error
	}{
		{name: "refusal first", errs: []error{refusal, errors.New(`the cluster "ghost" does not exist`)}},
		{name: "refusal last", errs: []error{errors.New(`the cluster "ghost" does not exist`), refusal}},
		{name: "several others", errs: []error{
			errors.New(`the cluster "ghost" does not exist`),
			refusal,
			status.Error(codes.NotFound, "machine set not found"),
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var joined error

			joined = multierror.Append(joined, test.errs...)

			err := validated.ValidationError(joined)

			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.ErrorContains(t, err, "insufficient role")
			require.NotContains(t, err.Error(), "ghost")
			require.NotContains(t, err.Error(), "machine set not found")
		})
	}
}

// TestValidationErrorKeepsEveryErrorWithoutARefusal checks that the errors of a request nobody refused are all reported.
func TestValidationErrorKeepsEveryErrorWithoutARefusal(t *testing.T) {
	var joined error

	joined = multierror.Append(joined,
		errors.New(`the cluster "ghost" does not exist`),
		status.Error(codes.NotFound, "machine set not found"),
	)

	err := validated.ValidationError(joined)

	require.ErrorContains(t, err, "ghost")
	require.ErrorContains(t, err, "machine set not found")
}
