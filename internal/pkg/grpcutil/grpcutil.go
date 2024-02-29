// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpcutil provides utilities for gRPC.
package grpcutil

import (
	"errors"
	"fmt"

	"google.golang.org/grpc"
)

// FullMethodName returns full method name for the given gRPC method from the given gRPC service.
func FullMethodName(serviceDesc *grpc.ServiceDesc, methodName string) (string, error) {
	if methodName == "" {
		return "", errors.New("method name is empty")
	}

	for _, method := range serviceDesc.Methods {
		if method.MethodName == methodName {
			return "/" + serviceDesc.ServiceName + "/" + method.MethodName, nil
		}
	}

	for _, method := range serviceDesc.Streams {
		if method.StreamName == methodName {
			return "/" + serviceDesc.ServiceName + "/" + method.StreamName, nil
		}
	}

	return "", fmt.Errorf("method %q not found in service %q", methodName, serviceDesc.ServiceName)
}

// MustFullMethodName is a helper function to get full method name for the given gRPC method from the given gRPC service.
// It panics if method is not found.
func MustFullMethodName(serviceDesc *grpc.ServiceDesc, methodName string) string {
	fullMethodName, err := FullMethodName(serviceDesc, methodName)
	if err != nil {
		panic(err)
	}

	return fullMethodName
}
