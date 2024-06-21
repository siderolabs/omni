// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package management provides client for Omni management API.
package management

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/management"
)

// TalosconfigOption is a functional option for Talosconfig.
type TalosconfigOption func(*management.TalosconfigRequest)

// WithBreakGlassTalosconfig sets whether the Talosconfig request should return an operator Talosconfig which bypasses Omni.
func WithBreakGlassTalosconfig(value bool) TalosconfigOption {
	return func(req *management.TalosconfigRequest) {
		req.BreakGlass = value
	}
}

// WithRawTalosconfig sets whether the Talosconfig request should return a raw Talosconfig.
func WithRawTalosconfig(value bool) TalosconfigOption {
	return func(req *management.TalosconfigRequest) {
		req.Raw = value
	}
}

// KubeconfigOption is a functional option for Kubeconfig.
type KubeconfigOption func(request *management.KubeconfigRequest)

// WithServiceAccount sets whether the Kubeconfig request should return a user or service account type kubeconfig.
func WithServiceAccount(ttl time.Duration, user string, groups ...string) KubeconfigOption {
	return func(request *management.KubeconfigRequest) {
		request.ServiceAccount = true
		request.ServiceAccountTtl = durationpb.New(ttl)
		request.ServiceAccountUser = user
		request.ServiceAccountGroups = groups
	}
}

// WithGrantType sets --grant-type in the generated kubeconfig.
func WithGrantType(grantType string) KubeconfigOption {
	return func(request *management.KubeconfigRequest) {
		request.GrantType = grantType
	}
}

// WithBreakGlassKubeconfig sets whether the Kubeconfig request should return an admin Kubeconfig.
func WithBreakGlassKubeconfig(value bool) KubeconfigOption {
	return func(request *management.KubeconfigRequest) {
		request.BreakGlass = value
	}
}

// Client for Management API .
type Client struct {
	conn management.ManagementServiceClient
}

// NewClient builds a client out of gRPC connection.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		conn: management.NewManagementServiceClient(conn),
	}
}

// WithCluster sets the cluster name context.
func (client *Client) WithCluster(clusterName string) *ClusterClient {
	return &ClusterClient{
		client:      client,
		clusterName: clusterName,
	}
}

// Talosconfig retrieves Talos client configuration for the whole instance.
func (client *Client) Talosconfig(ctx context.Context, opts ...TalosconfigOption) ([]byte, error) {
	request := management.TalosconfigRequest{}

	for _, opt := range opts {
		opt(&request)
	}

	talosconfigResp, err := client.conn.Talosconfig(ctx, &request)

	return talosconfigResp.GetTalosconfig(), err
}

// Omniconfig retrieves Omni configuration for the clients.
func (client *Client) Omniconfig(ctx context.Context) ([]byte, error) {
	omniconfig, err := client.conn.Omniconfig(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get omniconfig: %w", err)
	}

	return omniconfig.Omniconfig, nil
}

// LogsReader returns the io.Reader for the logs with each message separated by '\n'.
func (client *Client) LogsReader(ctx context.Context, machineID string, follow bool, tailLines int32) (io.Reader, error) {
	logStream, err := client.conn.MachineLogs(ctx, &management.MachineLogsRequest{
		MachineId: machineID,
		Follow:    follow,
		TailLines: tailLines,
	})
	if err != nil {
		return nil, err
	}

	return &LogReader{
		ctx:    ctx,
		client: logStream,
	}, nil
}

// CreateSchematic using the image factory.
func (client *Client) CreateSchematic(ctx context.Context, req *management.CreateSchematicRequest) (*management.CreateSchematicResponse, error) {
	schematic, err := client.conn.CreateSchematic(ctx, req)
	if err != nil {
		return nil, err
	}

	return schematic, nil
}

// CreateServiceAccount creates a service account and returns the public key ID.
func (client *Client) CreateServiceAccount(ctx context.Context, name, armoredPGPPublicKey, role string, useUserRole bool) (string, error) {
	resp, err := client.conn.CreateServiceAccount(ctx, &management.CreateServiceAccountRequest{
		ArmoredPgpPublicKey: armoredPGPPublicKey,
		Role:                role,
		UseUserRole:         useUserRole,
		Name:                name,
	})
	if err != nil {
		return "", err
	}

	return resp.PublicKeyId, nil
}

// RenewServiceAccount renews a service account and returns the public key ID.
func (client *Client) RenewServiceAccount(ctx context.Context, name, armoredPGPPublicKey string) (string, error) {
	resp, err := client.conn.RenewServiceAccount(ctx, &management.RenewServiceAccountRequest{
		Name:                name,
		ArmoredPgpPublicKey: armoredPGPPublicKey,
	})
	if err != nil {
		return "", err
	}

	return resp.PublicKeyId, nil
}

// ListServiceAccounts lists service accounts.
func (client *Client) ListServiceAccounts(ctx context.Context) ([]*management.ListServiceAccountsResponse_ServiceAccount, error) {
	response, err := client.conn.ListServiceAccounts(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.GetServiceAccounts(), nil
}

// DestroyServiceAccount deletes a service account.
func (client *Client) DestroyServiceAccount(ctx context.Context, name string) error {
	_, err := client.conn.DestroyServiceAccount(ctx, &management.DestroyServiceAccountRequest{
		Name: name,
	})

	return err
}

// GetSupportBundle generates support bundle on Omni server and returns it to the client.
func (client *Client) GetSupportBundle(ctx context.Context, cluster string, progress chan *management.GetSupportBundleResponse_Progress) ([]byte, error) {
	if progress != nil {
		defer close(progress)
	}

	serv, err := client.conn.GetSupportBundle(ctx, &management.GetSupportBundleRequest{
		Cluster: cluster,
	})
	if err != nil {
		return nil, err
	}

	for {
		msg, err := serv.Recv()
		if err != nil {
			return nil, err
		}

		if msg.BundleData != nil {
			return msg.BundleData, nil
		}

		if progress == nil {
			continue
		}

		progress <- msg.Progress
	}
}

// LogReader is a log client reader which implements io.Reader.
type LogReader struct {
	ctx    context.Context //nolint:containedctx
	client management.ManagementService_MachineLogsClient

	buf bytes.Buffer
}

// Read reads from the log stream.
func (l *LogReader) Read(p []byte) (int, error) {
	if l.buf.Len() > 0 {
		return l.buf.Read(p)
	}

	for {
		if l.ctx.Err() != nil {
			return 0, io.EOF
		}

		recv, err := l.client.Recv()
		if err != nil {
			if expectedErr(err) {
				return 0, io.EOF
			}

			return 0, err
		}

		err = writeLine(&l.buf, recv.Bytes)
		if err != nil {
			return 0, fmt.Errorf("failed to write log msg: %w", err)
		}

		if l.buf.Len() > 0 {
			return l.buf.Read(p)
		}
	}
}

type byteWriter interface {
	io.Writer
	io.ByteWriter
}

func writeLine(writer byteWriter, line []byte) error {
	_, err := writer.Write(line)
	if err != nil {
		return err
	}

	err = writer.WriteByte('\n')
	if err != nil {
		return err
	}

	return nil
}

func expectedErr(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) ||
		status.Code(err) == codes.Canceled
}

// ClusterClient is a client for a specific cluster.
type ClusterClient struct {
	client      *Client
	clusterName string
}

// Kubeconfig retrieves Kubernetes client configuration for the cluster.
func (client *ClusterClient) Kubeconfig(ctx context.Context, opts ...KubeconfigOption) ([]byte, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "context", client.clusterName)

	request := management.KubeconfigRequest{}

	for _, opt := range opts {
		opt(&request)
	}

	kubeconfigResp, err := client.client.conn.Kubeconfig(ctx, &request)

	return kubeconfigResp.GetKubeconfig(), err
}

// Talosconfig retrieves Talos client configuration for the cluster.
func (client *ClusterClient) Talosconfig(ctx context.Context, opts ...TalosconfigOption) ([]byte, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "context", client.clusterName)

	request := management.TalosconfigRequest{}

	for _, opt := range opts {
		opt(&request)
	}

	talosconfigResp, err := client.client.conn.Talosconfig(ctx, &request)

	return talosconfigResp.GetTalosconfig(), err
}

// KubernetesUpgradePreChecks runs the pre-checks for an upgrade.
func (client *ClusterClient) KubernetesUpgradePreChecks(ctx context.Context, newVersion string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "context", client.clusterName)

	resp, err := client.client.conn.KubernetesUpgradePreChecks(ctx, &management.KubernetesUpgradePreChecksRequest{
		NewVersion: newVersion,
	})
	if err != nil {
		return err
	}

	if resp.Ok {
		return nil
	}

	return fmt.Errorf("%s", resp.GetReason())
}

// KubernetesSyncManifestHandler is called for each sync event.
type KubernetesSyncManifestHandler func(*management.KubernetesSyncManifestResponse) error

// KubernetesSyncManifests syncs the bootstrap Kubernetes manifests.
func (client *ClusterClient) KubernetesSyncManifests(ctx context.Context, dryRun bool, handler KubernetesSyncManifestHandler) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = metadata.AppendToOutgoingContext(ctx, "context", client.clusterName)

	cli, err := client.client.conn.KubernetesSyncManifests(ctx, &management.KubernetesSyncManifestRequest{
		DryRun: dryRun,
	})
	if err != nil {
		return err
	}

	for {
		msg, err := cli.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
				return nil
			}

			return err
		}

		err = handler(msg)
		if err != nil {
			return err
		}
	}
}
