// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package backend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend"
)

func TestClientAddress(t *testing.T) {
	for _, test := range []struct {
		name         string
		remoteAddr   string
		expected     string
		forwardedFor []string
	}{
		{name: "public connection", remoteAddr: "203.0.113.9:4321", expected: "203.0.113.9"},
		{name: "public connection, header ignored", remoteAddr: "203.0.113.9:4321", forwardedFor: []string{"1.2.3.4"}, expected: "203.0.113.9"},
		{name: "one proxy", remoteAddr: "192.168.3.120:4321", forwardedFor: []string{"203.0.113.9"}, expected: "203.0.113.9"},
		{name: "two private hops", remoteAddr: "192.168.3.120:4321", forwardedFor: []string{"203.0.113.9, 10.20.0.5"}, expected: "203.0.113.9"},
		{name: "two header lines", remoteAddr: "192.168.3.120:4321", forwardedFor: []string{"203.0.113.9", "10.20.0.5"}, expected: "203.0.113.9"},
		{name: "proxy without a header", remoteAddr: "192.168.3.120:4321", expected: "192.168.3.120"},
		{name: "health probe", remoteAddr: "192.168.0.1:4321", expected: "192.168.0.1"},
		{name: "entry with a port", remoteAddr: "10.0.0.5:4321", forwardedFor: []string{"203.0.113.9:51000"}, expected: "203.0.113.9"},
		{name: "unparseable entry stops the walk", remoteAddr: "10.0.0.5:4321", forwardedFor: []string{"unknown, 10.0.0.9"}, expected: "10.0.0.9"},
		{name: "empty entry stops the walk", remoteAddr: "10.0.0.5:4321", forwardedFor: []string{"203.0.113.9, , 10.0.0.9"}, expected: "10.0.0.9"},
		{name: "mapped shared address space hop", remoteAddr: "192.168.3.120:4321", forwardedFor: []string{"203.0.113.9, ::ffff:100.64.1.1"}, expected: "203.0.113.9"},
		{name: "ipv6 connection", remoteAddr: "[2001:db8::1]:4321", forwardedFor: []string{"1.2.3.4"}, expected: "2001:db8::1"},
		{name: "ipv6 private hop", remoteAddr: "[fd00::1]:4321", forwardedFor: []string{"2001:db8::7"}, expected: "2001:db8::7"},
		{name: "shared address space hop", remoteAddr: "100.64.1.1:4321", forwardedFor: []string{"203.0.113.9"}, expected: "203.0.113.9"},
		{name: "walk ends at the leftmost entry", remoteAddr: "127.0.0.1:4321", forwardedFor: []string{"10.1.2.50, 10.1.2.3"}, expected: "10.1.2.50"},
		{name: "no connection address", remoteAddr: "", expected: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, backend.ClientAddress(test.remoteAddr, test.forwardedFor))
		})
	}
}

// TestSetClientAddress checks the x-forwarded-for metadata on both the REST gateway and the native gRPC path.
func TestSetClientAddress(t *testing.T) {
	newRequest := func() *http.Request {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/some.Service/Method", nil)
		req.RemoteAddr = "192.168.3.120:4321"
		req.Header.Add("X-Forwarded-For", "1.2.3.4") // client-sent
		req.Header.Add("X-Forwarded-For", "203.0.113.9")
		req.Header.Add("Grpc-Metadata-X-Forwarded-For", "7.7.7.7")
		req.Header.Add("Grpc-Metadata-X-Forwarded-For", "8.8.8.8")

		return req
	}

	req := newRequest()
	backend.SetClientAddress(req)

	assert.Equal(t, []string{"203.0.113.9"}, req.Header.Values("X-Forwarded-For"))
	assert.Empty(t, req.Header.Values("Grpc-Metadata-X-Forwarded-For"))

	// the gateway appends the connection address
	gatewayCtx, err := runtime.AnnotateContext(t.Context(), runtime.NewServeMux(), req, "/some.Service/Method")
	require.NoError(t, err)

	md, _ := metadata.FromOutgoingContext(gatewayCtx)
	assert.Equal(t, []string{"203.0.113.9, 192.168.3.120"}, md.Get("x-forwarded-for"))

	// the native gRPC server passes the header as it is
	var native metadata.MD

	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		native, _ = metadata.FromIncomingContext(stream.Context())

		return status.Error(codes.Unimplemented, "")
	}))

	req = newRequest()
	req.ProtoMajor, req.Proto = 2, "HTTP/2.0"
	req.Header.Set("Content-Type", "application/grpc")
	backend.SetClientAddress(req)

	server.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, []string{"203.0.113.9"}, native.Get("x-forwarded-for"))
}
