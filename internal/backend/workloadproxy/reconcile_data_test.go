// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func TestReconcileData(t *testing.T) {
	// Test empty
	var d *workloadproxy.ReconcileData

	require.Zero(t, d.GetHosts())
	require.Zero(t, d.PortForAlias("alias"))

	for range d.AliasesData() {
		t.Fatal("unexpected range")
	}

	d = &workloadproxy.ReconcileData{
		AliasPort: map[string]string{
			"alias1": "8080",
			"alias2": "8081",
		},
		Hosts: []string{
			"192.168.1.1",
			"192.168.1.2",
		},
	}

	require.Equal(t, []string{"192.168.1.1", "192.168.1.2"}, d.GetHosts())

	for als := range d.AliasesData() {
		switch als {
		case "alias1":
			require.Equal(t, "8080", d.PortForAlias(als))
		case "alias2":
			require.Equal(t, "8081", d.PortForAlias(als))
		default:
			t.Fatalf("unexpected alias %q", als)
		}
	}
}
