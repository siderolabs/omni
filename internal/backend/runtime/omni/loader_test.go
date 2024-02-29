// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni"
)

func TestNewLoader(t *testing.T) {
	type args struct {
		source string
	}

	tests := map[string]struct {
		want func(*testing.T, omni.Loader, error)
		pre  func(*testing.T)
		args args
	}{
		"empty source": {
			args: args{
				source: "",
			},
			want: errString("private key source is not set"),
		},
		"file loader": {
			args: args{
				source: "file:///some/file/private/key",
			},
			want: equalTo(&omni.FileLoader{Filepath: "/some/file/private/key"}),
		},
		"file loader with empty file": {
			args: args{
				source: "file://",
			},
			want: errString("file source is not set"),
		},
		"vault http no params": {
			args: args{
				source: "vault://",
			},
			want: errString("unknown private key source 'vault://'"),
		},
		"vault http with empty mount": {
			args: args{
				source: "vault:///some/path",
			},
			want: errString("unknown private key source 'vault:///some/path'"),
		},
		"vault http with empty secretPath": {
			args: args{
				source: "vault://some/",
			},
			want: errString("unknown private key source 'vault://some/'"),
		},
		"vault http no env set": {
			args: args{
				source: "vault://mount/some-path",
			},
			want: errString("VAULT_TOKEN is not set"),
		},
		"vault http not fully env set": {
			args: args{
				source: "vault://mount/some-path",
			},
			pre: func(t *testing.T) {
				t.Setenv("VAULT_TOKEN", "some-token")
			},
			want: errString("VAULT_ADDR is not set"),
		},
		"vault http": {
			args: args{
				source: "vault://mount/some-path",
			},
			pre: func(t *testing.T) {
				t.Setenv("VAULT_TOKEN", "some-token")
				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
			},
			want: equalTo(&omni.VaultHTTPLoader{
				Token:      "some-token",
				Address:    "http://127.0.0.1:8200",
				Mount:      "mount",
				SecretPath: "some-path",
			}),
		},
		"incorrect k8s vault": {
			args: args{
				source: "vault://k8s-role://omni-account-etcdEnc",
			},
			want: errString("unknown private key source 'vault://k8s-role://omni-account-etcdEnc'"),
		},
		"vault-k8s empty role": {
			args: args{
				source: "vault://:/secret/omni-account-etcdEnc",
			},
			want: errString("unknown private key source 'vault://:/secret/omni-account-etcdEnc'"),
		},
		"k8s vault": {
			pre: func(t *testing.T) {
				t.Setenv("VAULT_K8S_ROLE", "k8s-role")
			},
			args: args{
				source: "vault://secret/omni/account/etcdEnc",
			},
			want: equalTo(&omni.VaultK8sLoader{
				Role:       "k8s-role",
				Mount:      "secret",
				SecretPath: "omni/account/etcdEnc",
			}),
		},
		"k8s vault with path": {
			pre: func(t *testing.T) {
				t.Setenv("VAULT_K8S_ROLE", "k8s-role")
			},
			args: args{
				source: "vault://@/path/to/token:/secret/omni/account/etcdEnc",
			},
			want: equalTo(&omni.VaultK8sLoader{
				Role:       "k8s-role",
				TokenPath:  "/path/to/token",
				Mount:      "secret",
				SecretPath: "omni/account/etcdEnc",
			}),
		},
		"unknown source": {
			args: args{
				source: "vault-k9s://my-role:/path/to/token",
			},
			want: errString("unknown private key source 'vault-k9s://my-role:/path/to/token'"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.pre != nil {
				tt.pre(t)
			}

			got, err := omni.NewLoader(tt.args.source, zaptest.NewLogger(t))
			tt.want(t, got, err)
		})
	}
}

func equalTo(expected omni.Loader) func(*testing.T, omni.Loader, error) {
	return func(t *testing.T, actual omni.Loader, err error) {
		require.NoError(t, err)

		if !cmp.Equal(expected, actual, IgnoreUnexported(expected)) {
			t.Fatal(cmp.Diff(expected, actual, IgnoreUnexported(expected)))
		}
	}
}

func errString(expectedErr string) func(*testing.T, omni.Loader, error) {
	return func(t *testing.T, _ omni.Loader, err error) {
		require.EqualError(t, err, expectedErr)
	}
}

func IgnoreUnexported(vals ...any) cmp.Option {
	return cmpopts.IgnoreUnexported(xslices.Map(vals, func(v any) any {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		return val.Interface()
	})...)
}
