// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package secretrotation includes helpers for secret rotation.
package secretrotation

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// RemoteGenerator provides a client for accessing trustd.
type RemoteGenerator interface {
	IdentityContext(ctx context.Context, csr *talosx509.CertificateSigningRequest) (ca, crt []byte, err error)
	Close() error
}

// RemoteGeneratorFactory is the factory for providing a client for accessing trustd.
type RemoteGeneratorFactory interface {
	NewRemoteGenerator(token string, endpoints []string, acceptedCAs []*talosx509.PEMEncodedCertificate) (RemoteGenerator, error)
}

// Candidates is a list of candidates for rotation.
type Candidates struct {
	Candidates []Candidate
}

// Add adds a candidate to the list.
func (c *Candidates) Add(candidate Candidate) {
	c.Candidates = append(c.Candidates, candidate)
}

// Len returns the number of candidates.
func (c *Candidates) Len() int {
	return len(c.Candidates)
}

// Sort sorts the candidates by control plane, ready, and hostname.
func (c *Candidates) Sort() {
	slices.SortFunc(c.Candidates, func(a, b Candidate) int {
		switch {
		case a.Less(b):
			return -1
		case b.Less(a):
			return +1
		default:
			return 0
		}
	})
}

type Filter int

const (
	Parallel Filter = iota
	Serial
)

// Viable returns two slices of candidates after applying the given filters:
// This function takes Machine's type, readiness, and lock status into account.
//   - Candidates that are viable for secret rotation
//   - Candidates that require secret rotation but blocked until some requirements are met
func (c *Candidates) Viable(controlPlaneFilter Filter, workerFilter Filter) (viable []Candidate, blocked []Candidate) {
	var (
		viableCP  []Candidate
		viableW   []Candidate
		blockedCP []Candidate
		blockedW  []Candidate
	)

	c.Sort()

	for _, candidate := range c.Candidates {
		if !candidate.Ready || candidate.Locked {
			if candidate.ControlPlane {
				blockedCP = append(blockedCP, candidate)
			} else {
				blockedW = append(blockedW, candidate)
			}

			continue
		}

		if candidate.ControlPlane {
			viableCP = append(viableCP, candidate)

			continue
		}

		viableW = append(viableW, candidate)
	}

	switch controlPlaneFilter {
	case Parallel:
		viable = append(viable, viableCP...)
		blocked = append(blocked, blockedCP...)
	case Serial:
		for i, candidate := range viableCP {
			if i == 0 {
				viable = append(viable, candidate)

				continue
			}

			blocked = append(blocked, candidate)
		}

		blocked = append(blocked, blockedCP...)
	}

	if len(viable) > 0 || len(blocked) > 0 {
		return viable, blocked
	}

	switch workerFilter {
	case Parallel:
		viable = append(viable, viableW...)
		blocked = append(blocked, blockedW...)
	case Serial:
		for i, candidate := range viableW {
			if i == 0 {
				viable = append(viable, candidate)

				continue
			}

			blocked = append(blocked, candidate)
		}

		blocked = append(blocked, blockedW...)
	}

	return viable, blocked
}

// Locked returns the list of blocked machines. It returns a slice of control planes if there are locked control planes, otherwise it returns a slice of locked workers.
// This method call is informational. We first want to inform the caller about control planes, because having the knowledge of locked workers doesn't bring any benefit.
func (c *Candidates) Locked() []string {
	return c.filter(func(candidate Candidate) bool {
		return candidate.Locked
	})
}

// NotReady returns the list of machines that are not ready. It returns a slice of control planes if there are non-ready control planes, otherwise it returns a slice of non-ready workers.
// This method call is informational. We first want to inform the caller about control planes, because having the knowledge of unhealthy workers doesn't bring any benefit.
func (c *Candidates) NotReady() []string {
	return c.filter(func(candidate Candidate) bool {
		return !candidate.Ready
	})
}

func (c *Candidates) filter(filterFunc func(candidate Candidate) bool) []string {
	var cp, w []string

	for _, candidate := range c.Candidates {
		if filterFunc(candidate) {
			if candidate.ControlPlane {
				cp = append(cp, candidate.Hostname)
			} else {
				w = append(w, candidate.Hostname)
			}
		}
	}

	if len(cp) > 0 {
		return cp
	}

	return w
}

// Candidate is a candidate for rotation.
type Candidate struct {
	RemoteGeneratorFactory RemoteGeneratorFactory
	MachineID              string
	Hostname               string
	ControlPlane           bool
	Locked                 bool
	Ready                  bool
}

// Less returns true if the candidate should be rotated before the other one.
func (c Candidate) Less(other Candidate) bool {
	if c.ControlPlane != other.ControlPlane {
		return c.ControlPlane
	}

	if c.Ready != other.Ready {
		return c.Ready
	}

	if c.Locked != other.Locked {
		return !c.Locked
	}

	return c.Hostname < other.Hostname
}

func (c Candidate) Validate(ctx context.Context, status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (bool, error) {
	switch secrets.TypedSpec().Value.Rotation.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		return c.validateTalosCARotation(ctx, status, secrets)
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}

	return false, nil
}

func (c Candidate) validateTalosCARotation(ctx context.Context, status *omni.ClusterMachineStatus, cmSecrets *omni.ClusterMachineSecrets) (bool, error) {
	talosClient, err := c.getTalosClient(ctx, status, cmSecrets)
	if err != nil {
		return false, err
	}
	defer talosClient.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = talosClient.Version(ctx)
	if err != nil {
		return false, err
	}

	if c.ControlPlane {
		return c.checkTrustdGeneratedCerts(ctx, talosClient, status, cmSecrets)
	}

	return true, nil
}

func (c Candidate) getTalosClient(ctx context.Context, status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (*client.Client, error) {
	address := status.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	ca, clientCert, err := c.talosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.MakeSet(role.Admin))
	if err != nil {
		return nil, err
	}

	config := &clientconfig.Config{
		Context: status.Metadata().ID(),
		Contexts: map[string]*clientconfig.Context{
			status.Metadata().ID(): {
				Endpoints: endpoints,
				CA:        base64.StdEncoding.EncodeToString(ca),
				Crt:       base64.StdEncoding.EncodeToString(clientCert.Crt),
				Key:       base64.StdEncoding.EncodeToString(clientCert.Key),
			},
		},
	}

	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", status.Metadata().ID(), err)
	}

	return result, nil
}

func (c Candidate) talosAPIClientCertificateFromSecrets(secrets *omni.ClusterMachineSecrets, validity time.Duration, roles role.Set) ([]byte, *talosx509.PEMEncodedCertificateAndKey, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, err
	}

	switch secrets.TypedSpec().Value.Rotation.Phase {
	case specs.SecretRotationSpec_PRE_ROTATE:
		newCA := &talosx509.PEMEncodedCertificateAndKey{
			Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.Os.Crt,
			Key: secrets.TypedSpec().Value.Rotation.ExtraCerts.Os.Key,
		}

		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), newCA, roles, validity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return secretsBundle.Certs.OS.Crt, clientCert, nil

	case specs.SecretRotationSpec_ROTATE, specs.SecretRotationSpec_POST_ROTATE:
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), secretsBundle.Certs.OS, roles, validity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return secretsBundle.Certs.OS.Crt, clientCert, nil

	case specs.SecretRotationSpec_OK:
		// nothing to do
	}

	return nil, nil, fmt.Errorf("unknown rotation phase: %s", secrets.TypedSpec().Value.Rotation.Phase)
}

func (c Candidate) getTrustdClient(status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (RemoteGenerator, []*talosx509.PEMEncodedCertificate, error) {
	endpoint := status.TypedSpec().Value.ManagementAddress

	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse secrets bundle: %w", err)
	}

	acceptedCAs := []*talosx509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.OS.Crt}}

	if secrets.TypedSpec().Value.Rotation.ExtraCerts.GetOs() != nil {
		acceptedCAs = append(acceptedCAs, &talosx509.PEMEncodedCertificate{Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.GetOs().Crt})
	}

	remoteGen, err := c.RemoteGeneratorFactory.NewRemoteGenerator(secretsBundle.TrustdInfo.Token, []string{endpoint}, acceptedCAs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating trustd client: %w", err)
	}

	return remoteGen, acceptedCAs, nil
}

func (c Candidate) checkTrustdGeneratedCerts(ctx context.Context, talosClient *client.Client, status *omni.ClusterMachineStatus, cmSecrets *omni.ClusterMachineSecrets) (bool, error) {
	trustdClient, acceptedCAs, err := c.getTrustdClient(status, cmSecrets)
	if err != nil {
		return false, err
	}
	defer trustdClient.Close() //nolint:errcheck

	certSAN, err := safe.ReaderGetByID[*secrets.CertSAN](ctx, talosClient.COSI, secrets.CertSANAPIID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, fmt.Errorf("certSAN resource not found: %w", err)
		}

		return false, fmt.Errorf("error getting certSANs: %w", err)
	}

	certSANs := certSAN.TypedSpec()

	acceptedCA, err := acceptedCAs[len(acceptedCAs)-1].GetCert()
	if err != nil {
		return false, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	serverCSR, serverCert, err := talosx509.NewCSRAndIdentityFromCA(
		acceptedCA,
		talosx509.IPAddresses(certSANs.StdIPs()),
		talosx509.DNSNames(certSANs.DNSNames),
		talosx509.CommonName(certSANs.FQDN),
	)
	if err != nil {
		return false, fmt.Errorf("failed to generate API server CSR: %w", err)
	}

	var serverCA []byte

	serverCA, serverCert.Crt, err = trustdClient.IdentityContext(ctx, serverCSR)
	if err != nil {
		return false, err
	}

	clientCA, clientCert, err := c.talosAPIClientCertificateFromSecrets(cmSecrets, constants.CertificateValidityTime, role.MakeSet(role.Admin))
	if err != nil {
		return false, err
	}

	verifyErr := c.verifyCert(serverCert.Crt, clientCA, x509.ExtKeyUsageServerAuth)
	if verifyErr != nil {
		return false, fmt.Errorf("trustd: failed to verify server cert: %w", verifyErr)
	}

	verifyErr = c.verifyCert(clientCert.Crt, serverCA, x509.ExtKeyUsageClientAuth)
	if verifyErr != nil {
		return false, fmt.Errorf("trustd: failed to verify client cert: %w", verifyErr)
	}

	return true, nil
}

func (c Candidate) verifyCert(certPEM, caPEM []byte, extKeyUsage x509.ExtKeyUsage) error {
	block, _ := pem.Decode(certPEM)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse cert: %w", err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caPEM); !ok {
		return fmt.Errorf("failed to append CA to pool")
	}

	opts := x509.VerifyOptions{
		Roots:         caPool,
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{extKeyUsage},
	}

	_, err = cert.Verify(opts)

	return err
}
