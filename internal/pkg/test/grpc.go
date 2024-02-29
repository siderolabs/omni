// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package test contains the test helpers.
package test

import (
	"fmt"
	"net"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
)

// GRPCSuite is a test suite that provides a gRPC server and client.
type GRPCSuite struct {
	suite.Suite

	listener net.Listener
	Server   *grpc.Server

	Target string
}

// InitServer initializes the test gRPC server.
//
// Options must b provided at this step, and servers must be registered before calling StartServer.
func (suite *GRPCSuite) InitServer(opts ...grpc.ServerOption) {
	var err error

	suite.listener, err = net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.Server = grpc.NewServer(opts...)

	addr, ok := suite.listener.Addr().(*net.TCPAddr)
	suite.Require().True(ok)

	suite.Target = fmt.Sprintf("localhost:%d", addr.Port)
}

// StartServer starts the test gRPC server.
//
// This method must be called after registering all the servers.
func (suite *GRPCSuite) StartServer() {
	go suite.Server.Serve(suite.listener) //nolint:errcheck
}

// StopServer stops the test gRPC server.
func (suite *GRPCSuite) StopServer() {
	suite.Server.Stop()
	suite.listener.Close() //nolint:errcheck
}
