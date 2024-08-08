// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

const (
	// Auth0 is auth0 confirmation type.
	Auth0 = "auth0"
	// SAML is SAML confirmation type.
	SAML = "saml"
)

// Data contains the audit data.
type Data struct {
	NewUser       *NewUser       `json:"new_user,omitempty"`
	Machine       *Machine       `json:"machine,omitempty"`
	MachineLabels *MachineLabels `json:"machine_labels,omitempty"`
	AccessPolicy  *AccessPolicy  `json:"access_policy,omitempty"`
	Cluster       *Cluster       `json:"cluster,omitempty"`
	Session       Session        `json:"session,omitempty"`
}

// Session contains information about the current session.
type Session struct {
	UserAgent           string    `json:"user_agent,omitempty"`
	IPAddress           string    `json:"ip_address,omitempty"`
	UserID              string    `json:"user_id,omitempty"`
	Role                role.Role `json:"role,omitempty"`
	Email               string    `json:"email,omitempty"`
	Fingerprint         string    `json:"fingerprint,omitempty"`
	ConfirmationType    string    `json:"confirmation_type,omitempty"`
	PublicKeyExpiration int64     `json:"public_key_expiration,omitempty"`
}

// NewUser contains information about the new user.
type NewUser struct {
	Role             role.Role `json:"role,omitempty"`
	UserID           string    `json:"id,omitempty"`
	Email            string    `json:"email,omitempty"`
	IsServiceAccount bool      `json:"is_service_account,omitempty"`
}

// Machine contains information about the machine.
type Machine struct {
	Labels            map[string]string `json:"labels,omitempty"`
	ID                string            `json:"id,omitempty"`
	ManagementAddress string            `json:"management_address,omitempty"`
	IsConnected       bool              `json:"is_connected,omitempty"`
}

// MachineLabels contains information about the machine labels.
type MachineLabels struct {
	Labels map[string]string `json:"labels,omitempty"`
	ID     string            `json:"id,omitempty"`
}

// AccessPolicy contains information about the access policy.
type AccessPolicy struct {
	ID            resource.ID                                `json:"id,omitempty"`
	ClusterGroups map[string]*specs.AccessPolicyClusterGroup `json:"cluster_groups,omitempty"`
	UserGroups    map[string]*specs.AccessPolicyUserGroup    `json:"user_groups,omitempty"`
	Rules         []*specs.AccessPolicyRule                  `json:"rules,omitempty"`
	Tests         []*specs.AccessPolicyTest                  `json:"tests,omitempty"`
}

// Cluster struct contains information about the cluster.
type Cluster struct {
	ID                  string                      `json:"id,omitempty"`
	Labels              map[string]string           `json:"labels,omitempty"`
	BackupConfiguration *specs.EtcdBackupConf       `json:"backup_configuration,omitempty"`
	Features            *specs.ClusterSpec_Features `json:"features,omitempty"`
	KubernetesVersion   string                      `json:"kubernetes_version,omitempty"`
	TalosVersion        string                      `json:"talos_version,omitempty"`
}
