// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"net"
	"slices"
	"syscall"
	"time"

	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
)

type remote struct {
	Logger   *zap.Logger
	Addr     string
	AddrPort string
}

func (r *remote) Equal(other *remote) bool { return r.AddrPort == other.AddrPort }

func (r *remote) HealthCheck(ctx context.Context) (upstream.Tier, error) {
	start := time.Now()
	err := r.healthCheck(ctx)
	elapsed := time.Since(start)

	return calcTier(err, elapsed)
}

func (r *remote) healthCheck(ctx context.Context) error {
	d := probeDialer()

	c, err := d.DialContext(ctx, "tcp", r.AddrPort)
	if err != nil {
		r.Logger.Debug("healthcheck failed", zap.String("address", r.AddrPort), zap.Error(err))

		return err
	}

	return c.Close()
}

var times = []time.Duration{
	0,
	time.Millisecond / 10,
	time.Millisecond,
	10 * time.Millisecond,
	100 * time.Millisecond,
	1 * time.Second,
}

func calcTier(err error, elapsed time.Duration) (upstream.Tier, error) {
	if err == nil {
		for i := range slices.Backward(times) {
			if elapsed >= times[i] {
				return upstream.Tier(i), nil
			}
		}
	}

	// preserve old tier
	return -1, err
}

func probeDialer() *net.Dialer {
	return &net.Dialer{
		// The dialer reduces the TIME-WAIT period to 1 seconds instead of the OS default of 60 seconds.
		Control: func(_, _ string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 1}) //nolint: errcheck
			})
		},
	}
}
