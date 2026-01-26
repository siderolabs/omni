// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package trustd

import (
	"bytes"
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"net"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

type handler struct {
	securityapi.UnimplementedSecurityServiceServer

	state  state.State
	logger *zap.Logger
}

func getSecrets(ctx context.Context, st state.State, peerAddress string) (*omni.ClusterSecrets, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	machines, err := safe.StateListAll[*omni.Machine](
		ctx,
		st,
		state.WithLabelQuery(resource.LabelEqual(omni.MachineAddressLabel, peerAddress)),
	)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "failed to list machines: %s", err)
	}

	if machines.Len() != 1 {
		return nil, status.Errorf(codes.PermissionDenied, "failed to find machine with address %s", peerAddress)
	}

	machine := machines.Get(0)

	clusterMachine, err := safe.StateGet[*omni.ClusterMachine](ctx, st, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, machine.Metadata().ID(), resource.VersionUndefined))
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "failed to get cluster machine: %s", err)
	}

	clusterID, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "failed to get cluster ID from cluster machine %s", machine.Metadata().ID())
	}

	grpc_ctxtags.Extract(ctx).Set("cluster", clusterID)

	clusterSecrets, err := safe.StateGet[*omni.ClusterSecrets](ctx, st, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterSecretsType, clusterID, resource.VersionUndefined))
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "failed to get cluster secrets: %s", err)
	}

	return clusterSecrets, nil
}

// Certificate implements the securityapi.SecurityServer interface.
//
// This API is called by Talos worker nodes to request a server certificate for apid running on the node.
// Control plane nodes generate certificates (client and server) directly from machine config PKI.
func (h *handler) Certificate(ctx context.Context, in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	remotePeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "peer not found")
	}

	tcpAddr, ok := remotePeer.Addr.(*net.TCPAddr)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "peer address is not TCP")
	}

	clusterSecrets, err := getSecrets(ctx, h.state, tcpAddr.IP.String())
	if err != nil {
		return nil, err
	}

	secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
	if err != nil {
		return nil, err
	}

	issuingCA, acceptedCAs, err := GetIssuingAndAcceptedCAs(ctx, h.state, secretsBundle, clusterSecrets.Metadata().ID())
	if err != nil {
		return nil, err
	}

	// validate the token
	md, _ := metadata.FromIncomingContext(ctx)
	if token := md.Get("token"); len(token) != 1 || token[0] != secretsBundle.TrustdInfo.Token {
		return nil, status.Error(codes.PermissionDenied, "invalid token")
	}

	// decode and validate CSR
	csrPemBlock, _ := pem.Decode(in.Csr)
	if csrPemBlock == nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to decode CSR")
	}

	request, err := stdx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse CSR: %s", err)
	}

	h.logger.Info("received CSR signing request",
		zap.Stringer("remote_addr", remotePeer.Addr),
		zap.Stringer("subject", request.Subject),
		zap.Strings("dns_names", request.DNSNames),
		zap.Stringers("ip_adresses", request.IPAddresses),
	)

	// allow only server auth certificates
	x509Opts := []x509.Option{
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	}

	// don't allow any certificates which can be used for client authentication
	if len(request.Subject.Organization) > 0 {
		return nil, status.Error(codes.PermissionDenied, "organization field is not allowed in CSR")
	}

	signed, err := x509.NewCertificateFromCSRBytes(
		issuingCA.Crt,
		issuingCA.Key,
		in.Csr,
		x509Opts...,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to sign CSR: %s", err)
	}

	resp = &securityapi.CertificateResponse{
		Ca: bytes.Join(
			xslices.Map(
				acceptedCAs,
				func(cert *x509.PEMEncodedCertificate) []byte {
					return cert.Crt
				},
			),
			nil,
		),
		Crt: signed.X509CertificatePEM,
	}

	return resp, nil
}

func GetIssuingAndAcceptedCAs(ctx context.Context, st state.State, secretsBundle *secrets.Bundle, clusterID resource.ID) (*x509.PEMEncodedCertificateAndKey, []*x509.PEMEncodedCertificate, error) {
	secretRotation, err := safe.ReaderGetByID[*omni.SecretRotation](ctx, st, clusterID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, nil, err
	}

	acceptedCAs := []*x509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.OS.Crt}}
	issuingCA := secretsBundle.Certs.OS

	// If there is an active secret rotation, then we need to use SecretRotation resource to construct issuingCA and acceptedCAs for trustd to use. ClusterSecrets will receive updates
	// after a phase is completed, but trustd needs to be immediately aware of the rotation to issue certificates correctly.
	if secretRotation != nil && secretRotation.TypedSpec().Value.Component == specs.SecretRotationSpec_TALOS_CA {
		acceptedCAs = []*x509.PEMEncodedCertificate{{Crt: secretRotation.TypedSpec().Value.Certs.Os.Crt}}
		if secretRotation.TypedSpec().Value.ExtraCerts.GetOs() != nil {
			acceptedCAs = append(acceptedCAs, &x509.PEMEncodedCertificate{Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt})
		}

		switch secretRotation.TypedSpec().Value.Phase {
		case specs.SecretRotationSpec_OK, specs.SecretRotationSpec_POST_ROTATE:
			issuingCA = &x509.PEMEncodedCertificateAndKey{
				Crt: secretRotation.TypedSpec().Value.Certs.Os.Crt,
				Key: secretRotation.TypedSpec().Value.Certs.Os.Key,
			}

		case specs.SecretRotationSpec_PRE_ROTATE, specs.SecretRotationSpec_ROTATE:
			issuingCA = &x509.PEMEncodedCertificateAndKey{
				Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt,
				Key: secretRotation.TypedSpec().Value.ExtraCerts.Os.Key,
			}
		}
	}

	return issuingCA, acceptedCAs, nil
}
