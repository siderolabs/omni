// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // Register the proto codec before we replace it with ours.
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

// Name is the name registered for the proto compressor.
const Name = "proto"

type vtprotoCodec struct{}

func (c vtprotoCodec) Marshal(v any) (mem.BufferSlice, error) {
	size, err := getSize(v)
	if err != nil {
		return nil, err
	}

	if mem.IsBelowBufferPoolingThreshold(size) {
		buf, err := marshal(v)
		if err != nil {
			return nil, err
		}

		return mem.BufferSlice{mem.SliceBuffer(buf)}, nil
	}

	pool := mem.DefaultBufferPool()

	buf := pool.Get(size)
	if err := marshalAppend((*buf)[:size], v); err != nil {
		pool.Put(buf)

		return nil, err
	}

	return mem.BufferSlice{mem.NewBuffer(buf, pool)}, nil
}

func getSize(v any) (int, error) {
	switch v := v.(type) {
	case vtprotoMessage:
		return v.SizeVT(), nil
	case proto.Message:
		return proto.Size(v), nil
	case protoadapt.MessageV1:
		return proto.Size(protoadapt.MessageV2Of(v)), nil
	default:
		return -1, fmt.Errorf("failed to get size, message is %T, must satisfy the vtprotoMessage, proto.Message or protoadapt.MessageV1 ", v)
	}
}

func marshal(v any) ([]byte, error) {
	switch v := v.(type) {
	case vtprotoMessage:
		return v.MarshalVT()
	case proto.Message:
		return proto.Marshal(v)
	case protoadapt.MessageV1:
		return proto.Marshal(protoadapt.MessageV2Of(v))
	default:
		return nil, fmt.Errorf("failed to marshal, message is %T, must satisfy the vtprotoMessage, proto.Message or protoadapt.MessageV1 ", v)
	}
}

func marshalAppend(dst []byte, v any) error {
	takeErr := func(_ any, e error) error { return e }

	switch v := v.(type) {
	case vtprotoMessage:
		return takeErr(v.MarshalToSizedBufferVT(dst))
	case proto.Message:
		return takeErr((proto.MarshalOptions{}).MarshalAppend(dst, v))
	case protoadapt.MessageV1:
		return takeErr((proto.MarshalOptions{}).MarshalAppend(dst[:0], protoadapt.MessageV2Of(v)))
	default:
		return fmt.Errorf("failed to marshal-append, message is %T, must satisfy the vtprotoMessage, proto.Message or protoadapt.MessageV1 ", v)
	}
}

func (c vtprotoCodec) Unmarshal(data mem.BufferSlice, v any) error {
	buf := data.MaterializeToBuffer(mem.DefaultBufferPool())
	defer buf.Free()

	switch v := v.(type) {
	case vtprotoMessage:
		return v.UnmarshalVT(buf.ReadOnlyData())
	case proto.Message:
		return proto.Unmarshal(buf.ReadOnlyData(), v)
	case protoadapt.MessageV1:
		return proto.Unmarshal(buf.ReadOnlyData(), protoadapt.MessageV2Of(v))
	default:
		return fmt.Errorf("failed to unmarshal, message is %T, must satisfy the vtprotoMessage, proto.Message or protoadapt.MessageV1", v)
	}
}

func (c vtprotoCodec) Name() string { return Name }

func (vtprotoCodec) OldName() string { return Name }

type vtprotoMessage interface {
	MarshalToSizedBufferVT([]byte) (int, error)
	MarshalVT() ([]byte, error)
	UnmarshalVT([]byte) error
	SizeVT() int
}

func init() { encoding.RegisterCodecV2(vtprotoCodec{}) }
