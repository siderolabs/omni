// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package loadbalancer

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	minAcceptRetryDelay = 5 * time.Millisecond
	maxAcceptRetryDelay = time.Second
)

type listener struct {
	inner     net.Listener
	accepted  chan net.Conn
	done      chan struct{}
	inProcess map[*trackedConn]struct{}
	mu        sync.Mutex
	closed    bool
}

func newListener(inner net.Listener) *listener {
	l := &listener{
		inner:     inner,
		accepted:  make(chan net.Conn),
		done:      make(chan struct{}),
		inProcess: map[*trackedConn]struct{}{},
	}

	go l.forward()

	return l
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.accepted:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *listener) Close() error {
	l.mu.Lock()

	if l.closed {
		l.mu.Unlock()

		return nil
	}

	l.closed = true
	close(l.done)

	conns := make([]*trackedConn, 0, len(l.inProcess))
	for conn := range l.inProcess {
		conns = append(conns, conn)
	}

	l.mu.Unlock()

	err := l.inner.Close()

	for _, conn := range conns {
		conn.Close() //nolint:errcheck
	}

	return err
}

func (l *listener) Addr() net.Addr {
	return l.inner.Addr()
}

func (l *listener) dial(ctx context.Context) (net.Conn, error) {
	client, server := net.Pipe()

	tracked, ok := l.track(server)
	if !ok {
		client.Close() //nolint:errcheck
		server.Close() //nolint:errcheck

		return nil, ErrNotRunning
	}

	select {
	case l.accepted <- tracked:
		return client, nil
	case <-l.done:
	case <-ctx.Done():
	}

	client.Close()  //nolint:errcheck
	tracked.Close() //nolint:errcheck

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return nil, ErrNotRunning
}

func (l *listener) track(conn net.Conn) (*trackedConn, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, false
	}

	tracked := &trackedConn{Conn: conn}
	tracked.release = func() { l.untrack(tracked) }

	l.inProcess[tracked] = struct{}{}

	return tracked, true
}

func (l *listener) untrack(conn *trackedConn) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.inProcess, conn)
}

func (l *listener) forward() {
	var delay time.Duration

	for {
		conn, err := l.inner.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			delay = min(max(2*delay, minAcceptRetryDelay), maxAcceptRetryDelay)

			select {
			case <-time.After(delay):
				continue
			case <-l.done:
				return
			}
		}

		delay = 0

		select {
		case l.accepted <- conn:
		case <-l.done:
			conn.Close() //nolint:errcheck

			return
		}
	}
}

type trackedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(c.release)

	return c.Conn.Close()
}
