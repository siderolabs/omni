// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package ratelimit throttles COSI mutations (Create/Update/Destroy) against the
// etcd backend by marshaled-payload size, per caller class.
//
// Throttler is wired as the state-etcd WithLimiter hook: each mutation blocks
// until enough bytes-budget is available for the marshaled payload, bounded by
// min(ctx.Deadline-now, maxWait). On timeout it returns codes.DeadlineExceeded.
// Service accounts and unknown callers share the `user` bucket.
package ratelimit

import (
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

const (
	bucketInternal      = 0
	bucketInfraProvider = 1
	bucketUser          = 2
	bucketCount         = 3
)

// bucketLabels are the Prometheus `class` label values, mirroring actor.Type names.
var bucketLabels = [bucketCount]string{
	bucketInternal:      string(actor.TypeInternal),
	bucketInfraProvider: string(actor.TypeInfraProvider),
	bucketUser:          string(actor.TypeUser),
}

func bucketFor(t actor.Type) int {
	switch t { //nolint:exhaustive
	case actor.TypeInternal:
		return bucketInternal
	case actor.TypeInfraProvider:
		return bucketInfraProvider
	}

	return bucketUser
}
