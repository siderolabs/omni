// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // Register the proto codec before we replace it with ours.
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

// Name is the name registered for the proto compressor.
const Name = "proto"

type vtprotoCodec struct{}

func (vtprotoCodec) Marshal(v any) ([]byte, error) {
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

func (vtprotoCodec) Unmarshal(data []byte, v any) error {
	switch v := v.(type) {
	case vtprotoMessage:
		return v.UnmarshalVT(data)
	case proto.Message:
		return proto.Unmarshal(data, v)
	case protoadapt.MessageV1:
		return proto.Unmarshal(data, protoadapt.MessageV2Of(v))
	default:
		return fmt.Errorf("failed to unmarshal, message is %T, must satisfy the vtprotoMessage, proto.Message or protoadapt.MessageV1", v)
	}
}

func (vtprotoCodec) Name() string { return Name }

type vtprotoMessage interface {
	MarshalVT() ([]byte, error)
	UnmarshalVT([]byte) error
}

func init() { encoding.RegisterCodec(vtprotoCodec{}) }
