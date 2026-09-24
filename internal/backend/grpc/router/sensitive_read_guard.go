// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"context"
	"reflect"
	"sync/atomic"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// frameType is the type of the raw messages the proxy forwards without decoding them.
var frameType = reflect.TypeOf(proxy.NewFrame(nil))

// readRequest is a COSI read request, which names the resource type it reads.
type readRequest interface {
	proto.Message
	GetType() string
}

// sensitiveReadGuard returns a client interceptor for the connections to the Talos nodes, which denies the COSI
// resource reads of sensitive types.
//
// Talos denies them itself for the roles Omni forwards, but not on its maintenance API, which grants every SideroLink
// peer, Omni included, all the roles: the reads would run as os:admin there. The guard decodes the resource type out of
// the request the proxy forwards and refuses the sensitive types on every node, as talos.CheckSensitivity does for the
// resource API, so that the denial never depends on the node.
func sensitiveReadGuard() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		var request readRequest

		switch method {
		case v1alpha1.State_Get_FullMethodName:
			request = &v1alpha1.GetRequest{}
		case v1alpha1.State_List_FullMethodName:
			request = &v1alpha1.ListRequest{}
		case v1alpha1.State_Watch_FullMethodName:
			request = &v1alpha1.WatchRequest{}
		default:
			return streamer(ctx, desc, cc, method, opts...)
		}

		ctx, cancel := context.WithCancel(ctx)

		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			cancel()

			return nil, err
		}

		return &guardedStream{ClientStream: stream, cancel: cancel, cc: cc, request: request}, nil
	}
}

// guardedStream checks the request before forwarding it. A denied request never reaches the node: the stream is
// canceled instead, and the receiver reports the denial, as the proxy returns the errors of that side to the caller as they are.
type guardedStream struct {
	grpc.ClientStream

	cancel  context.CancelFunc
	cc      *grpc.ClientConn
	request readRequest
	denied  atomic.Pointer[error]
	checked bool
}

func (s *guardedStream) SendMsg(m any) error {
	// only the raw frames the proxy forwards are checked, a typed message is a lookup of Omni's own
	if s.checked || reflect.TypeOf(m) != frameType {
		return s.ClientStream.SendMsg(m)
	}

	s.checked = true

	if err := s.check(m); err != nil {
		s.denied.Store(&err)
		s.cancel()

		return nil //nolint:nilerr // reported by RecvMsg
	}

	return s.ClientStream.SendMsg(m)
}

func (s *guardedStream) RecvMsg(m any) error {
	err := s.ClientStream.RecvMsg(m)

	if denied := s.denied.Load(); denied != nil {
		return *denied
	}

	return err
}

// check decodes the resource type out of the raw request and denies the sensitive types.
func (s *guardedStream) check(m any) error {
	// the frame keeps its payload private, marshaling it through the proxy codec is the way to get the bytes back
	data, err := proxy.Codec().Marshal(m)
	if err != nil {
		return err
	}

	payload := data.Materialize()

	data.Free()

	if err = proto.Unmarshal(payload, s.request); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to decode the request: %v", err)
	}

	node := state.WrapCore(client.NewAdapter(v1alpha1.NewStateClient(protoCodecConn{s.cc})))

	return talos.CheckSensitivity(s.Context(), node, s.request.GetType())
}

// protoCodecConn makes typed calls over a connection which forwards raw messages by default.
type protoCodecConn struct {
	*grpc.ClientConn
}

func (c protoCodecConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return c.ClientConn.Invoke(ctx, method, args, reply, append(opts, grpc.ForceCodecV2(encoding.GetCodecV2(grpcproto.Name)))...)
}

func (c protoCodecConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.ClientConn.NewStream(ctx, desc, method, append(opts, grpc.ForceCodecV2(encoding.GetCodecV2(grpcproto.Name)))...)
}
