// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package logreceiver provides a TCP server that receives logs from Talos.
package logreceiver

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/siderolabs/gen/containers"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

// Server implements TCP server to receive JSON logs. It is similar to the one in Talos, except it doesn't try to parse
// the JSON and instead writes rawdata and uses newline as a separator.
//
//nolint:govet
type Server struct {
	listener net.Listener
	handler  *ConnHandler
	logger   *zap.Logger

	closeOnce sync.Once
	// wg is used to wait for all connections to be closed.
	wg sync.WaitGroup

	m containers.ConcurrentMap[string, net.Conn]
}

// Handler handles a message or an error.
type Handler interface {
	HandleMessage(srcAddress netip.Addr, rawData []byte)
	HandleError(srcAddress netip.Addr, err error)
}

// NewServer initializes new Server.
func NewServer(listener net.Listener, handler *ConnHandler, logger *zap.Logger) *Server {
	return &Server{
		listener: listener,
		handler:  handler,
		logger:   logger,
	}
}

// Serve runs the TCP server loop.
func (srv *Server) Serve() error {
	for {
		conn, err := srv.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			return fmt.Errorf("error accepting connection: %w", err)
		}

		srcAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		if !ok {
			srv.logger.Error("error getting remote IP address")

			continue
		}

		remoteAddr, _ := netip.AddrFromSlice(srcAddr.IP)
		remoteAddress := conn.RemoteAddr().String()

		srv.wg.Add(1)
		srv.m.Set(remoteAddress, conn)

		panichandler.Go(func() {
			defer srv.wg.Done()
			srv.handler.HandleConn(remoteAddr, conn)
			srv.m.Remove(remoteAddress)
		}, srv.logger)
	}
}

// Stop serving.
func (srv *Server) Stop() {
	srv.closeOnce.Do(func() {
		srv.listener.Close() //nolint:errcheck

		srv.m.ForEach(func(_ string, conn net.Conn) {
			remotrAddr := conn.RemoteAddr().String()
			srv.logger.Info("closing connection", zap.String("remote_addr", remotrAddr))

			err := conn.Close()
			if err != nil {
				srv.logger.Warn("error closing connection", zap.String("remote_addr", remotrAddr), zap.Error(err))
			}
		})

		srv.logger.Info("waiting for goroutines to finish")
		// Wait for all connections goroutines to finish.
		srv.wg.Wait()
		srv.m.Clear()
		srv.logger.Info("stopped logging server")
	})
}

// ConnHandler is called for each received connection.
type ConnHandler struct {
	msgHandler Handler
	logger     *zap.Logger
}

// NewConnHandler initializes new ConnHandler.
func NewConnHandler(msgHandler Handler, logger *zap.Logger) *ConnHandler {
	return &ConnHandler{
		msgHandler: msgHandler,
		logger:     logger,
	}
}

// HandleConn handles a connection.
func (ch *ConnHandler) HandleConn(addr netip.Addr, conn io.ReadCloser) {
	defer conn.Close() //nolint:errcheck

	bufReader := bufio.NewReader(conn)

	for {
		slice, err := bufReader.ReadSlice('\n')
		if err != nil {
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !isTimeout(err) {
				ch.logger.Error("error decoding message", zap.Error(err))
				ch.msgHandler.HandleError(addr, err)
			}

			return
		}

		ch.msgHandler.HandleMessage(addr, slice[:len(slice)-1])
	}
}

func isTimeout(err error) bool {
	var neterr net.Error
	if errors.As(err, &neterr) {
		return neterr != nil && neterr.Timeout()
	}

	return false
}

// MakeServer creates a listener on the given address and returns a struct which can be used to start and stop the server.
func MakeServer(address string, handler Handler, logger *zap.Logger) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("log server: error listening on %s: %w", address, err)
	}

	return NewServer(listener, NewConnHandler(handler, logger), logger), nil
}
