// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

func TestDisconnectedBelowFloor(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		spec                 *specs.SiderolinkSpec
		name                 string
		checkPeerConnection  bool
		disconnectBelowFloor bool
		expected             bool
		expectErr            bool
	}{
		{name: "dropped below floor", spec: &specs.SiderolinkSpec{Connected: true, TalosVersion: "v1.5.0"}, checkPeerConnection: true, disconnectBelowFloor: true, expected: true},
		{name: "dropped without version", spec: &specs.SiderolinkSpec{Connected: true}, checkPeerConnection: true, disconnectBelowFloor: true, expected: true},
		{name: "already disconnected", spec: &specs.SiderolinkSpec{TalosVersion: "v1.5.0"}, checkPeerConnection: true, disconnectBelowFloor: true},
		{name: "at floor", spec: &specs.SiderolinkSpec{Connected: true, TalosVersion: "v1.6.0"}, checkPeerConnection: true, disconnectBelowFloor: true},
		{name: "grace period", spec: &specs.SiderolinkSpec{Connected: true, TalosVersion: "v1.5.0"}, disconnectBelowFloor: true},
		{name: "floor not enforced", spec: &specs.SiderolinkSpec{Connected: true, TalosVersion: "v1.5.0"}, checkPeerConnection: true},
		{name: "invalid version", spec: &specs.SiderolinkSpec{Connected: true, TalosVersion: "not-a-version"}, checkPeerConnection: true, disconnectBelowFloor: true, expectErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			disconnected, err := siderolink.DisconnectedBelowFloor(tt.spec, tt.checkPeerConnection, tt.disconnectBelowFloor)
			if tt.expectErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, disconnected)
		})
	}
}
