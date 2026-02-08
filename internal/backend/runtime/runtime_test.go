// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

//nolint:iface
func TestInstall_LookupInterface(t *testing.T) {
	k8s, err := kubernetes.New(nil, "", "", "")
	require.NoError(t, err)

	runtime.Install(kubernetes.Name, k8s)

	type incorrectIface interface {
		GetClient(ctx context.Context, cluster string) (kubernetes.Client, error)
	}

	_, err = runtime.LookupInterface[incorrectIface](kubernetes.Name)
	require.EqualError(t, err, fmt.Sprintf("runtime with id %s is not incorrectIface", kubernetes.Name))

	type incorrectUnnamedIface = interface {
		GetClient(ctx context.Context, cluster string) (kubernetes.Client, error)
	}

	_, err = runtime.LookupInterface[incorrectUnnamedIface](kubernetes.Name)
	require.EqualError(t, err, fmt.Sprintf("runtime with id %s is not interface { GetClient(context.Context, string) (kubernetes.Client, error) }", kubernetes.Name))

	type correctIface = interface {
		GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
	}

	_, err = runtime.LookupInterface[correctIface](kubernetes.Name)
	require.NoError(t, err)
}
