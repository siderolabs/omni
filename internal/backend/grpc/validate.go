// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/internal/pkg/auth"
	omnijsonschema "github.com/siderolabs/omni/internal/pkg/jsonschema"
)

type publicKey struct {
	expiration time.Time
	id         string
	username   string
	data       []byte
}

// validatePublicKey validates the public key in the request and returns a publicKey.
func validatePublicKey(keypb *authpb.PublicKey, opts ...pgp.ValidationOption) (publicKey, error) {
	if keypb.GetPgpData() == nil && keypb.GetWebauthnData() == nil {
		return publicKey{}, errors.New("no public key data provided")
	}

	if keypb.GetWebauthnData() != nil {
		return publicKey{}, status.Error(codes.Unimplemented, "unimplemented") // todo: implement webauthn
	}

	return validatePGPPublicKey(keypb.GetPgpData(), opts...)
}

func validatePGPPublicKey(armored []byte, opts ...pgp.ValidationOption) (publicKey, error) {
	pgpKey, err := pgpcrypto.NewKeyFromArmored(string(armored))
	if err != nil {
		return publicKey{}, err
	}

	key, err := pgp.NewKey(pgpKey)
	if err != nil {
		return publicKey{}, err
	}

	err = key.Validate(opts...)
	if err != nil {
		return publicKey{}, err
	}

	if key.IsPrivate() {
		return publicKey{}, errors.New("PGP key contains private key")
	}

	lifetimeSecs := pgpKey.GetEntity().PrimaryIdentity().SelfSignature.KeyLifetimeSecs
	if lifetimeSecs == nil {
		return publicKey{}, errors.New("PGP key has no expiration")
	}

	expiration := pgpKey.GetEntity().PrimaryKey.CreationTime.Add(time.Duration(*lifetimeSecs) * time.Second)

	return publicKey{
		data:       armored,
		id:         pgpKey.GetFingerprint(),
		username:   pgpKey.GetEntity().PrimaryIdentity().UserId.Name,
		expiration: expiration,
	}, nil
}

func (s *managementServer) ValidateJSONSchema(ctx context.Context, request *management.ValidateJsonSchemaRequest) (*management.ValidateJsonSchemaResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithValidSignature(true)); err != nil {
		return nil, err
	}

	if len(request.Schema) > 1e6 {
		return nil, fmt.Errorf("json schema can not be bigger than 1MB")
	}

	var err error

	schema, err := omnijsonschema.Parse("untitled", request.Schema)
	if err != nil {
		return nil, err
	}

	err = omnijsonschema.Validate(request.Data, schema)
	if err != nil {
		var validationError *jsonschema.ValidationError
		if !errors.As(err, &validationError) {
			return nil, err
		}

		res := &management.ValidateJsonSchemaResponse{}

		res.Errors = handleValidationErrors(validationError)

		return res, nil
	}

	return &management.ValidateJsonSchemaResponse{}, nil
}

// handleValidationErrors processes the validation errors and appends them to the response.
func handleValidationErrors(validationError *jsonschema.ValidationError) []*management.ValidateJsonSchemaResponse_Error {
	var res []*management.ValidateJsonSchemaResponse_Error

	formatError := func(k jsonschema.ErrorKind) string {
		p := message.NewPrinter(language.English)

		return k.LocalizedString(p)
	}

	for _, nestedError := range validationError.Causes {
		schemaPath := nestedError.SchemaURL
		dataPath := "/" + strings.Join(nestedError.InstanceLocation, "/")

		switch k := nestedError.ErrorKind.(type) {
		case *kind.Required:
			for _, path := range k.Missing {
				res = append(res, &management.ValidateJsonSchemaResponse_Error{
					SchemaPath: schemaPath,
					Cause:      "property is required",
					DataPath:   filepath.Join(dataPath, path),
				})
			}
		default:
			res = append(res, &management.ValidateJsonSchemaResponse_Error{
				SchemaPath: schemaPath,
				Cause:      formatError(k),
				DataPath:   dataPath,
			})
		}

		if len(nestedError.Causes) > 0 {
			// Recursively process any nested errors (if any)
			res = append(res, handleValidationErrors(nestedError)...)
		}
	}

	return res
}
