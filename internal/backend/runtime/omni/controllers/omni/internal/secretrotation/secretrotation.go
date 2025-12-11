// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package secretrotation includes helpers for secret rotation.
package secretrotation

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"time"

	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

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

// Blocked returns the list of blocked machines. It returns a slice of control planes if there are locked control planes, otherwise it returns a slice of locked workers.
// This method call is informational. We first want to inform the caller about control planes, because having the knowledge of locked workers doesn't bring any benefit.
func (c *Candidates) Blocked() []string {
	return c.filter(func(candidate Candidate) bool {
		return candidate.Blocked
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
	MachineID    string
	Hostname     string
	ControlPlane bool
	Blocked      bool
	Ready        bool
}

// Less returns true if the candidate should be rotated before the other one.
func (c Candidate) Less(other Candidate) bool {
	if c.ControlPlane != other.ControlPlane {
		return c.ControlPlane
	}

	if c.Ready != other.Ready {
		return c.Ready
	}

	if c.Blocked != other.Blocked {
		return !c.Blocked
	}

	return c.Hostname < other.Hostname
}

func (c Candidate) Validate(ctx context.Context, secrets *omni.ClusterSecrets, cmStatus *omni.ClusterMachineStatus) (bool, error) {
	switch secrets.TypedSpec().Value.ComponentInRotation {
	case specs.ClusterSecretsRotationStatusSpec_TALOS_CA:
		return c.validateTalosCARotation(ctx, secrets, cmStatus)
	case specs.ClusterSecretsRotationStatusSpec_NONE:
		// nothing to do
	}

	return false, nil
}

func (c Candidate) validateTalosCARotation(ctx context.Context, secrets *omni.ClusterSecrets, cmStatus *omni.ClusterMachineStatus) (bool, error) {
	talosClient, err := c.getTalosClient(ctx, secrets, cmStatus)
	if err != nil {
		return false, err
	}
	defer talosClient.Close() //nolint:errcheck

	_, err = talosClient.Version(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c Candidate) getTalosClient(
	ctx context.Context,
	secrets *omni.ClusterSecrets,
	cmStatus *omni.ClusterMachineStatus,
) (*client.Client, error) {
	address := cmStatus.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	clientCert, CA, err := c.talosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.MakeSet(role.Admin))
	if err != nil {
		return nil, err
	}

	config := &clientconfig.Config{
		Context: cmStatus.Metadata().ID(),
		Contexts: map[string]*clientconfig.Context{
			cmStatus.Metadata().ID(): {
				Endpoints: endpoints,
				CA:        base64.StdEncoding.EncodeToString(CA),
				Crt:       base64.StdEncoding.EncodeToString(clientCert.Crt),
				Key:       base64.StdEncoding.EncodeToString(clientCert.Key),
			},
		},
	}
	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", cmStatus.Metadata().ID(), err)
	}

	return result, nil
}

func (c Candidate) talosAPIClientCertificateFromSecrets(secrets *omni.ClusterSecrets, certificateValidity time.Duration, roles role.Set) (*talosx509.PEMEncodedCertificateAndKey, []byte, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, err
	}

	rotateSecretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetRotateData())
	if err != nil {
		return nil, nil, err
	}

	switch secrets.TypedSpec().Value.RotationPhase {
	case specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE:
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), rotateSecretsBundle.Certs.OS, roles, certificateValidity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return clientCert, secretsBundle.Certs.OS.Crt, nil

	case specs.ClusterSecretsRotationStatusSpec_ROTATE:
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), rotateSecretsBundle.Certs.OS, roles, certificateValidity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return clientCert, rotateSecretsBundle.Certs.OS.Crt, nil

	case specs.ClusterSecretsRotationStatusSpec_POST_ROTATE:
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), secretsBundle.Certs.OS, roles, certificateValidity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return clientCert, secretsBundle.Certs.OS.Crt, nil
	case specs.ClusterSecretsRotationStatusSpec_OK:
		// nothing to do
	}

	return nil, nil, fmt.Errorf("unknown rotation phase: %s", secrets.TypedSpec().Value.RotationPhase.String())
}
