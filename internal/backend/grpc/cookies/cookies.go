// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package cookies defines tools for adding setting cookies in the gRPC gateway response.
package cookies

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const cookiesMetaHeader = "grpc-gateway-cookies"

// Set allows setting HTTP cookies from the gRPC services, propagating it to the gRPC gateway through Headers.
func Set(ctx context.Context, cookies ...*http.Cookie) error {
	header := metadata.Pairs()

	for _, cookie := range cookies {
		data, err := json.Marshal(cookie)
		if err != nil {
			return err
		}

		header.Append(cookiesMetaHeader, string(data))
	}

	return grpc.SendHeader(ctx, header)
}

// Handler is called from the gRPC gateway to get the cookies from the headers and set them in the HTTP response.
func Handler(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to extract ServerMetadata from context")
	}

	cookies := md.HeaderMD.Get(cookiesMetaHeader)
	if len(cookies) == 0 {
		return nil
	}

	for _, encodedCookie := range cookies {
		var cookie http.Cookie

		if err := json.Unmarshal([]byte(encodedCookie), &cookie); err != nil {
			return err
		}

		http.SetCookie(w, &cookie)
	}

	return nil
}
