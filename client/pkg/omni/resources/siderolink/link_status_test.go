// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

func TestLinkStatusID(t *testing.T) {
	t.Parallel()

	link := siderolink.NewLink("machine-1", &specs.SiderolinkSpec{})
	linkStatus := siderolink.NewLinkStatus(link)

	assert.Equal(t, "machine-1/Links.omni.sidero.dev", linkStatus.Metadata().ID())
	assert.Equal(t, linkStatus.Metadata().ID(), siderolink.LinkStatusID("machine-1", siderolink.LinkType))

	id, typ, ok := siderolink.ParseLinkStatusID(linkStatus.Metadata().ID())
	assert.True(t, ok)
	assert.Equal(t, "machine-1", id)
	assert.Equal(t, siderolink.LinkType, typ)

	_, _, ok = siderolink.ParseLinkStatusID("machine-1")
	assert.False(t, ok)
}
