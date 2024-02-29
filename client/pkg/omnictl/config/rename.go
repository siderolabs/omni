// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "fmt"

// Rename describes context rename during merge.
type Rename struct {
	From string
	To   string
}

// String converts to "from" -> "to".
func (r *Rename) String() string {
	return fmt.Sprintf("%s -> %s", r.From, r.To)
}
