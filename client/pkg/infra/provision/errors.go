// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
)

// NewRetryError should be returned from the provisioner when it hits the retryable error.
func NewRetryError(err error, interval time.Duration) error {
	return controller.NewRequeueError(err, interval)
}

// NewRetryErrorf should be returned from the provisioner when it hits the retryable error.
func NewRetryErrorf(interval time.Duration, format string, args ...any) error {
	return controller.NewRequeueErrorf(interval, format, args...)
}

// NewRetryInterval should be returned from the provisioner when it should be called after some interval again.
func NewRetryInterval(interval time.Duration) error {
	return controller.NewRequeueInterval(interval)
}
