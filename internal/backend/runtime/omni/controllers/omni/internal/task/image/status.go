// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package image

import "github.com/siderolabs/omni/client/pkg/omni/resources/omni"

// PullStatusChan is a channel for sending PullStatus from tasks back to the controller.
type PullStatusChan chan<- PullStatus

// PullStatus represents the status of a pull operation.
type PullStatus struct {
	Error      error
	Request    *omni.ImagePullRequest
	Node       string
	Image      string
	CurrentNum int
	TotalNum   int
}
