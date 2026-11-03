// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package trustd provides "extra" virtual trustd for Talos workers.
//
// This trustd allows worker nodes to start apid even if the cluster control plane is down.
package trustd

import (
	"crypto/tls"
	stdlibx509 "crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/siderolabs/crypto/x509"
	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	grpc_ctxtags "github.com/siderolabs/omni/internal/pkg/grpcutil/grpctags"
	grpc_zap "github.com/siderolabs/omni/internal/pkg/grpcutil/grpczap"
)

func recoveryHandler(logger *zap.Logger) grpc_recovery.RecoveryHandlerFunc {
	return func(p any) error {
		if logger != nil {
			logger.Error("grpc panic", zap.Any("panic", p), zap.Stack("stack"))
		}

		return status.Errorf(codes.Internal, "internal error")
	}
}

// getCertificate provides dynamic server certificate for trustd based on the calling cluster member.
func getCertificate(st state.State, serverAddr net.IP) func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		tcpAddr, ok := info.Conn.RemoteAddr().(*net.TCPAddr)
		if !ok {
			return nil, errors.New("failed to get remote address")
		}

		clusterSecrets, err := getSecrets(info.Context(), st, tcpAddr.IP.String())
		if err != nil {
			return nil, err
		}

		secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
		if err != nil {
			return nil, err
		}

		return issueServerCertificate(secretsBundle.Certs.OS, serverAddr)
	}
}

// issueServerCertificate issues a short-lived trustd server certificate signed by the cluster OS CA.
//
// The certificate is marked with the trustd-specific extended key usage, same way as Talos does it for its trustd.
// Talos versions which don't check the extended key usage ignore it, so it is safe to always set it.
func issueServerCertificate(osCA *x509.PEMEncodedCertificateAndKey, serverAddr net.IP) (*tls.Certificate, error) {
	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(osCA)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	serverCert, err := x509.NewKeyPair(
		ca,
		x509.IPAddresses([]net.IP{serverAddr}),
		x509.CommonName("omni"),
		x509.NotAfter(time.Now().Add(5*time.Minute)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
		}),
		// mark the certificate as trustd server certificate
		x509.UnknownExtKeyUsage([]asn1.ObjectIdentifier{
			constants.TrustdServerExtKeyUsage,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API server cert: %w", err)
	}

	return serverCert.Certificate, nil
}

// NewServer initializes grpc server for trustd.
func NewServer(logger *zap.Logger, st state.State, serverAddr net.IP) *grpc.Server {
	recoveryOpt := grpc_recovery.WithRecoveryHandler(recoveryHandler(logger))

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_zap.UnaryServerInterceptor(logger),
		grpc_prometheus.UnaryServerInterceptor,
		grpc_recovery.UnaryServerInterceptor(recoveryOpt),
	}

	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_zap.StreamServerInterceptor(logger),
		grpc_prometheus.StreamServerInterceptor,
		grpc_recovery.StreamServerInterceptor(recoveryOpt),
	}

	tlsConfig := &tls.Config{
		MinVersion:     tls.VersionTLS13,
		GetCertificate: getCertificate(st, serverAddr),
		ClientAuth:     tls.NoClientCert,
	}

	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.Creds(
			credentials.NewTLS(tlsConfig),
		),
	}

	server := grpc.NewServer(options...)
	securityapi.RegisterSecurityServiceServer(server, &handler{
		logger: logger,
		state:  st,
	})

	return server
}
