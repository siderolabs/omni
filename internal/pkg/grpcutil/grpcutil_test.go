// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
package grpcutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/interop/grpc_testing"

	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

func TestGetFullMethodName(t *testing.T) {
	type args struct {
		methodName string
	}

	tests := []struct {
		want func(*testing.T, string, error)
		name string
		args args
	}{
		{
			name: "proper method name",
			args: args{
				methodName: "EmptyCall",
			},
			want: func(t *testing.T, actual string, err error) {
				require.NoError(t, err)
				require.Equal(t, "/grpc.testing.TestService/EmptyCall", actual)
			},
		},
		{
			name: "incorrect method name",
			args: args{
				methodName: "Empty2Call",
			},
			want: equalError(`method "Empty2Call" not found in service "grpc.testing.TestService"`),
		},
		{
			name: "empty method name",
			args: args{
				methodName: "",
			},
			want: equalError("method name is empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := grpcutil.FullMethodName(&grpc_testing.TestService_ServiceDesc, tt.args.methodName)
			tt.want(t, got, err)
		})
	}

	require.Panics(t, func() {
		grpcutil.MustFullMethodName(&grpc_testing.TestService_ServiceDesc, "Empty2Call")
	})

	require.NotPanics(t, func() {
		grpcutil.MustFullMethodName(&grpc_testing.TestService_ServiceDesc, "EmptyCall")
	})
}

func equalError(eq string) func(*testing.T, string, error) {
	return func(t *testing.T, _ string, err error) {
		require.Error(t, err)
		require.EqualError(t, err, eq)
	}
}
