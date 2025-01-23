// Copyright (c) 2025 Sidero Labs, Inc.
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
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/siderolabs/crypto/x509"
	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
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

		secretsBundle, err := getSecretsBundle(info.Context(), st, tcpAddr.IP.String())
		if err != nil {
			return nil, err
		}

		// issue a short-lived cert for trustd for this connection
		ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(secretsBundle.Certs.OS)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
		}

		serverCert, err := x509.NewKeyPair(ca,
			x509.IPAddresses([]net.IP{serverAddr}),
			x509.CommonName("omni"),
			x509.NotAfter(time.Now().Add(5*time.Minute)),
			x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
			x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
				stdlibx509.ExtKeyUsageServerAuth,
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate API server cert: %w", err)
		}

		return serverCert.Certificate, nil
	}
}

// NewServer initializes grpc server for trustd.
func NewServer(logger *zap.Logger, st state.State, serverAddr net.IP) *grpc.Server {
	recoveryOpt := grpc_recovery.WithRecoveryHandler(recoveryHandler(logger))

	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 1, 10, 30, 60, 120, 300, 600}))

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
		grpc.SharedWriteBuffer(true),
	}

	server := grpc.NewServer(options...)
	securityapi.RegisterSecurityServiceServer(server, &handler{
		logger: logger,
		state:  st,
	})

	return server
}
