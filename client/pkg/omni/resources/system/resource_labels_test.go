// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

func TestType(t *testing.T) {
	lt := system.ResourceLabelsType[*system.DBVersion]()

	require.Equal(t, "DBVersionLabels.system.sidero.dev", lt)
}
