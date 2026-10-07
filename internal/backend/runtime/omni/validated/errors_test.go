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
	permissionDenied := status.Error(codes.PermissionDenied, `insufficient role: "None"`)
	unauthenticated := status.Error(codes.Unauthenticated, "unauthenticated: missing valid signature")

	for _, test := range []struct {
		refusal error
		name    string
		errs    []error
	}{
		{name: "permission denied first", refusal: permissionDenied, errs: []error{permissionDenied, errors.New(`the cluster "ghost" does not exist`)}},
		{name: "permission denied last", refusal: permissionDenied, errs: []error{errors.New(`the cluster "ghost" does not exist`), permissionDenied}},
		{name: "unauthenticated first", refusal: unauthenticated, errs: []error{unauthenticated, errors.New(`the cluster "ghost" does not exist`)}},
		{name: "unauthenticated last", refusal: unauthenticated, errs: []error{errors.New(`the cluster "ghost" does not exist`), unauthenticated}},
		{name: "several others", refusal: permissionDenied, errs: []error{
			errors.New(`the cluster "ghost" does not exist`),
			permissionDenied,
			status.Error(codes.NotFound, "machine set not found"),
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var joined error

			joined = multierror.Append(joined, test.errs...)

			err := validated.ValidationError(joined)

			require.Equal(t, status.Code(test.refusal), status.Code(err))
			require.ErrorContains(t, err, status.Convert(test.refusal).Message())
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
