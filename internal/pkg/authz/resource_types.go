// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package authz decides what a caller may do in Omni.
package authz

import (
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	oidcres "github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// Verbs is a set of the verbs of the resource API.
type Verbs uint8

// The verbs of the resource API. Read covers get, list and watch.
const (
	Read Verbs = 1 << iota
	Create
	Update
	Destroy

	AllVerbs = Read | Create | Update | Destroy
)

// Has reports whether the set contains the verb.
func (v Verbs) Has(verb state.Verb) bool {
	switch verb {
	case state.Get, state.List, state.Watch:
		return v&Read != 0
	case state.Create:
		return v&Create != 0
	case state.Update:
		return v&Update != 0
	case state.Destroy:
		return v&Destroy != 0
	default:
		return false
	}
}

// String returns the verbs as a phrase, e.g., "read, update and destroy".
func (v Verbs) String() string {
	var names []string

	for _, verb := range []struct {
		name string
		verb Verbs
	}{{"read", Read}, {"create", Create}, {"update", Update}, {"destroy", Destroy}} {
		if v&verb.verb != 0 {
			names = append(names, verb.name)
		}
	}

	if len(names) < 2 {
		return strings.Join(names, "")
	}

	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// ClusterRelation is the way a resource belongs to a cluster.
type ClusterRelation int

const (
	// NoCluster is for the resources which do not belong to a cluster.
	NoCluster ClusterRelation = iota
	// ClusterByID is for the resources which have the ID of their cluster as their ID.
	ClusterByID
	// ClusterByLabel is for the resources which have the ID of their cluster in the cluster label.
	ClusterByLabel
	// ClusterByMachine is for the resources which have the ID of a machine as their ID.
	// Reading one is checked against the cluster the machine is a member of, if any.
	// Lists and writes are checked without a cluster.
	ClusterByMachine
)

// ResourceGroup is a group of resource types which the roles grant access to as a whole.
type ResourceGroup string

// The resource groups.
const (
	// Clusters are the clusters and everything which exists because of a cluster.
	Clusters ResourceGroup = "clusters"
	// ExposedServices are the services of the clusters which are exposed through the workload proxy.
	ExposedServices ResourceGroup = "exposed-services"
	// Machines are the resources describing a single machine.
	Machines ResourceGroup = "machines"
	// MachineManagement are the settings of the machines, and the resources to add machines to Omni: classes, requests and join tokens.
	MachineManagement ResourceGroup = "machine-management"
	// Providers are the infra providers and their resources.
	Providers ResourceGroup = "providers"
	// AccessControl are the users, their identities and what grants access, and the credentials Omni stores.
	AccessControl ResourceGroup = "access-control"
)

// Access is what a caller needs before the grants are looked at.
type Access int

const (
	// RequiresGrant is for the types which need a grant.
	RequiresGrant Access = iota
	// RequiresAuthentication is for the types every authenticated caller can read.
	RequiresAuthentication
	// Public is for the types everyone can read, authenticated or not.
	Public
)

// ResourceType describes a resource type for the access checks.
type ResourceType struct {
	// Group is the resource group of the type. It is empty for the types which need no grant, and for the ones no caller can access.
	Group ResourceGroup
	// GroupWithoutCluster is the group of a resource of the type which belongs to no cluster, when it differs from Group.
	// For example, a config patch can be for a cluster or for a machine. The one for a machine is a setting of that machine.
	GroupWithoutCluster ResourceGroup
	Cluster             ClusterRelation
	Access              Access
	// Verbs are the verbs a caller of the API can ever be allowed, whatever its grants are.
	Verbs Verbs
	// ProviderReadable is set for the types outside the infra provider namespaces which the infra providers read.
	//
	// An infra provider works with the resources in the infra provider namespaces, and the infra provider state checks
	// the access to those. To do its job it also reads a few other types, e.g., the parameters to connect a machine to Omni.
	// It has no grants, so this is what lets it read them.
	ProviderReadable bool
}

// LookupResourceType returns the description of the resource type.
//
// A type which is not in the table belongs to no cluster and allows no verbs.
func LookupResourceType(typ resource.Type) ResourceType {
	return resourceTypes[typ]
}

// resourceTypes describes every resource type a caller of the API can access.
//
// Each type which needs a grant is put into the one resource group which fits it best. The roles grant access
// to groups, so the group of a type decides which roles reach it.
var resourceTypes = map[resource.Type]ResourceType{
	authres.AccessPolicyType:                         {Verbs: AllVerbs, Group: AccessControl},
	authres.AuthConfigType:                           {Verbs: Read, Access: Public},
	authres.EulaAcceptanceType:                       {Verbs: Read, Access: RequiresAuthentication},
	authres.IdentityLabelRuleType:                    {Verbs: AllVerbs, Group: AccessControl},
	authres.IdentityLastActiveType:                   {Verbs: Read, Group: AccessControl},
	authres.IdentityStatusType:                       {Verbs: Read, Group: AccessControl},
	authres.IdentityType:                             {Verbs: Read, Group: AccessControl},
	authres.PublicKeyLastActiveType:                  {Verbs: Read, Group: AccessControl},
	authres.ServiceAccountStatusType:                 {Verbs: Read, Group: AccessControl},
	authres.UserType:                                 {Verbs: Read, Group: AccessControl},
	infra.InfraMachineStatusType:                     {Cluster: ClusterByLabel},
	infra.InfraMachineType:                           {Cluster: ClusterByLabel},
	infra.ProviderType:                               {Verbs: AllVerbs, Group: Providers},
	meta.NamespaceType:                               {Verbs: Read, Access: RequiresAuthentication},
	meta.ResourceDefinitionType:                      {Verbs: Read, Access: RequiresAuthentication},
	oidcres.JWTPublicKeyType:                         {Verbs: Read | Destroy, Group: AccessControl},
	omni.ClusterBootstrapStatusType:                  {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterConfigVersionType:                    {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterDestroyStatusType:                    {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterDiagnosticsType:                      {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterEndpointType:                         {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterKubernetesManifestsStatusType:        {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterKubernetesNodesType:                  {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterMachineConfigPatchesType:             {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineConfigStatusType:              {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineConfigType:                    {Cluster: ClusterByLabel},
	omni.ClusterMachineIdentityType:                  {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineRequestStatusType:             {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineSecretsType:                   {Cluster: ClusterByLabel},
	omni.ClusterMachineStatusType:                    {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineTalosVersionType:              {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMachineType:                          {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ClusterMetricsType:                          {Verbs: Read, Access: RequiresAuthentication},
	omni.ClusterSecretsRotationStatusType:            {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterSecretsType:                          {Cluster: ClusterByID},
	omni.ClusterStatusMetricsType:                    {Verbs: Read, Access: RequiresAuthentication},
	omni.ClusterStatusType:                           {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterTaintType:                            {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterType:                                 {Cluster: ClusterByID, Verbs: AllVerbs, Group: Clusters},
	omni.ClusterUUIDType:                             {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ClusterWorkloadProxyStatusType:              {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.ConfigPatchType:                             {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters, GroupWithoutCluster: MachineManagement},
	omni.ControlPlaneStatusType:                      {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.DiscoveryAffiliateDeleteTaskType:            {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.EtcdAuditResultType:                         {Cluster: ClusterByID},
	omni.EtcdBackupOverallStatusType:                 {Verbs: Read, Group: Clusters},
	omni.EtcdBackupS3ConfType:                        {Verbs: AllVerbs, Group: AccessControl},
	omni.EtcdBackupStatusType:                        {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.EtcdBackupStoreStatusType:                   {Verbs: Read, Group: Clusters},
	omni.EtcdBackupType:                              {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.EtcdManualBackupType:                        {Cluster: ClusterByID, Verbs: AllVerbs, Group: Clusters},
	omni.ExposedServiceType:                          {Cluster: ClusterByLabel, Verbs: Read, Group: ExposedServices},
	omni.ExtensionsConfigurationType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.FeaturesConfigType:                          {Verbs: Read, Access: RequiresAuthentication},
	omni.ImagePullRequestType:                        {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ImagePullStatusType:                         {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.ImportedClusterSecretsType:                  {Cluster: ClusterByID, Verbs: Create | Destroy, Group: Clusters},
	omni.InfraMachineBMCConfigType:                   {Verbs: AllVerbs, Group: AccessControl},
	omni.InfraMachineConfigType:                      {Verbs: AllVerbs, Group: MachineManagement},
	omni.InfraProviderCombinedStatusType:             {Verbs: Read, Group: Providers},
	omni.InstallationMediaConfigType:                 {Verbs: AllVerbs, Group: MachineManagement},
	omni.InstallationMediaType:                       {Verbs: Read, Access: RequiresAuthentication},
	omni.KernelArgsStatusType:                        {Cluster: ClusterByLabel, Verbs: Read, Group: Machines},
	omni.KernelArgsType:                              {Verbs: AllVerbs, Group: MachineManagement},
	omni.KubernetesHealthCheckStatusType:             {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.KubernetesHealthCheckType:                   {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.KubernetesManifestGroupType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.KubernetesNodeAuditResultType:               {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.KubernetesStatusType:                        {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.KubernetesUpgradeManifestStatusType:         {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.KubernetesUpgradeStatusType:                 {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.KubernetesVersionType:                       {Verbs: Read, Access: RequiresAuthentication},
	omni.LoadBalancerConfigType:                      {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.LoadBalancerStatusType:                      {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.MachineClassType:                            {Verbs: AllVerbs, Group: MachineManagement},
	omni.MachineConfigDiffType:                       {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.MachineConfigExtractionStatusType:           {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.MachineConfigGenOptionsType:                 {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.MachineExtensionsStatusType:                 {Cluster: ClusterByLabel, Verbs: Read, Group: Machines},
	omni.MachineExtensionsType:                       {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.MachineInstallDiskConfigType:                {Verbs: AllVerbs, Group: MachineManagement},
	omni.MachineInstallDiskStatusType:                {Cluster: ClusterByLabel, Verbs: Read, Group: Machines},
	omni.MachineLabelsType:                           {Verbs: AllVerbs, Group: MachineManagement},
	omni.MachinePendingUpdatesType:                   {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.MachineRequestSetType:                       {Verbs: AllVerbs, Group: MachineManagement},
	omni.MachineSetDestroyStatusType:                 {Verbs: Read, Group: Clusters},
	omni.MachineSetNodeType:                          {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.MachineSetStatusType:                        {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.MachineSetType:                              {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.MachineStatusLinkType:                       {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.MachineStatusMetricsType:                    {Verbs: Read, Access: RequiresAuthentication},
	omni.MachineStatusSnapshotType:                   {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.MachineStatusType:                           {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.MachineType:                                 {Cluster: ClusterByMachine, Verbs: Read, Group: Machines, ProviderReadable: true},
	omni.MachineUpgradeStatusType:                    {Cluster: ClusterByLabel, Verbs: Read, Group: Machines},
	omni.MaintenanceConfigStatusType:                 {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	omni.NodeForceDestroyRequestType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs, Group: Clusters},
	omni.NotificationType:                            {Verbs: Read, Access: RequiresAuthentication},
	omni.OngoingTaskType:                             {Verbs: Read, Access: RequiresAuthentication},
	omni.RedactedClusterMachineConfigType:            {Cluster: ClusterByLabel, Verbs: Read, Group: Clusters},
	omni.RotateKubernetesCAType:                      {Cluster: ClusterByID, Verbs: AllVerbs, Group: Clusters},
	omni.RotateTalosCAType:                           {Cluster: ClusterByID, Verbs: AllVerbs, Group: Clusters},
	omni.SchematicConfigurationType:                  {Cluster: ClusterByLabel, Verbs: Read, Group: Machines},
	omni.SecretRotationType:                          {Cluster: ClusterByID},
	omni.TalosExtensionsType:                         {Verbs: Read, Access: RequiresAuthentication},
	omni.TalosUpgradeStatusType:                      {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	omni.TalosVersionType:                            {Verbs: Read, Access: RequiresAuthentication},
	omni.UpgradeRolloutType:                          {Cluster: ClusterByID, Verbs: Read, Group: Clusters},
	siderolink.APIConfigType:                         {Verbs: Read, Group: MachineManagement, ProviderReadable: true},
	siderolink.ConnectionParamsType:                  {Verbs: Read, Group: MachineManagement, ProviderReadable: true},
	siderolink.DefaultJoinTokenType:                  {Verbs: AllVerbs, Group: MachineManagement},
	siderolink.GRPCTunnelConfigType:                  {Verbs: AllVerbs, Group: MachineManagement},
	siderolink.JoinTokenStatusType:                   {Verbs: Read, Group: MachineManagement},
	siderolink.JoinTokenType:                         {Verbs: Read | Update | Destroy, Group: MachineManagement},
	siderolink.LinkStatusType:                        {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	siderolink.LinkType:                              {Cluster: ClusterByMachine, Verbs: Read | Destroy, Group: Machines},
	siderolink.NodeUniqueTokenStatusType:             {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	siderolink.PendingMachineType:                    {Verbs: Read | Destroy, Group: MachineManagement},
	system.ResourceLabelsType[*omni.MachineStatus](): {Cluster: ClusterByMachine, Verbs: Read, Group: Machines},
	system.SysVersionType:                            {Verbs: Read, Access: RequiresAuthentication},
	virtual.AdvertisedEndpointsType:                  {Verbs: Read, Access: RequiresAuthentication},
	virtual.CloudPlatformConfigType:                  {Verbs: Read, Access: RequiresAuthentication},
	virtual.ClusterPermissionsType:                   {Verbs: Read, Access: RequiresAuthentication},
	virtual.CurrentUserType:                          {Verbs: Read, Access: RequiresAuthentication},
	virtual.KubernetesUsageType:                      {Verbs: Read, Group: Clusters},
	virtual.LabelsCompletionType:                     {Verbs: Read, Access: RequiresAuthentication},
	virtual.MetalPlatformConfigType:                  {Verbs: Read, Access: RequiresAuthentication},
	virtual.PermissionsType:                          {Verbs: Read, Access: RequiresAuthentication},
	virtual.QuirksType:                               {Verbs: Read, Access: RequiresAuthentication},
	virtual.SBCConfigType:                            {Verbs: Read, Access: RequiresAuthentication},
	virtual.SupportType:                              {Verbs: Read, Access: RequiresAuthentication},
	virtual.VersionContractType:                      {Verbs: Read, Access: RequiresAuthentication},
}
