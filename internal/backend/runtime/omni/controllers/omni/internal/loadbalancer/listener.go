// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package loadbalancer

import (
	"context"
	"net"
	"sync"
)

type listener struct {
	inner net.Listener
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newListener(inner net.Listener) *listener {
	l := &listener{
		inner: inner,
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}

	go l.forward()

	return l
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *listener) Close() error {
	var err error

	l.once.Do(func() {
		close(l.done)

		err = l.inner.Close()
	})

	return err
}

func (l *listener) Addr() net.Addr {
	return l.inner.Addr()
}

func (l *listener) dial(ctx context.Context) (net.Conn, error) {
	client, server := net.Pipe()

	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
	case <-ctx.Done():
	}

	client.Close() //nolint:errcheck
	server.Close() //nolint:errcheck

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return nil, ErrNotRunning
}

func (l *listener) forward() {
	for {
		conn, err := l.inner.Accept()
		if err != nil {
			return
		}

		select {
		case l.conns <- conn:
		case <-l.done:
			conn.Close() //nolint:errcheck

			return
		}
	}
}
