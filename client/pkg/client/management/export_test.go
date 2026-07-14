// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package management

import (
	"time"

	"github.com/siderolabs/omni/client/api/omni/management"
)

// NewTestClient builds a client from the given raw service client, for tests, reconnecting
// follow streams without a real-time floor.
func NewTestClient(conn management.ManagementServiceClient) *Client {
	return &Client{conn: conn, followReconnectFloor: time.Millisecond}
}
