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
)

// ResourceType describes a resource type for the access checks.
type ResourceType struct {
	Cluster ClusterRelation
	// Verbs are the verbs a caller of the API can ever be allowed, whatever its role is.
	Verbs Verbs
}

// LookupResourceType returns the description of the resource type.
//
// A type which is not in the table belongs to no cluster and allows no verbs.
func LookupResourceType(typ resource.Type) ResourceType {
	return resourceTypes[typ]
}

var resourceTypes = map[resource.Type]ResourceType{
	authres.AccessPolicyType:                         {Verbs: AllVerbs},
	authres.AuthConfigType:                           {Verbs: Read},
	authres.EulaAcceptanceType:                       {Verbs: Read},
	authres.IdentityLabelRuleType:                    {Verbs: AllVerbs},
	authres.IdentityLastActiveType:                   {Verbs: Read},
	authres.IdentityStatusType:                       {Verbs: Read},
	authres.IdentityType:                             {Verbs: Read},
	authres.PublicKeyLastActiveType:                  {Verbs: Read},
	authres.ServiceAccountStatusType:                 {Verbs: Read},
	authres.UserType:                                 {Verbs: Read},
	infra.InfraMachineStatusType:                     {Cluster: ClusterByLabel},
	infra.InfraMachineType:                           {Cluster: ClusterByLabel},
	infra.ProviderType:                               {Verbs: AllVerbs},
	meta.NamespaceType:                               {Verbs: Read},
	meta.ResourceDefinitionType:                      {Verbs: Read},
	oidcres.JWTPublicKeyType:                         {Verbs: Read | Destroy},
	omni.ClusterBootstrapStatusType:                  {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterConfigVersionType:                    {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterDestroyStatusType:                    {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterDiagnosticsType:                      {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterEndpointType:                         {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterKubernetesManifestsStatusType:        {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterKubernetesNodesType:                  {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterMachineConfigPatchesType:             {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineConfigStatusType:              {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineConfigType:                    {Cluster: ClusterByLabel},
	omni.ClusterMachineIdentityType:                  {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineRequestStatusType:             {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineSecretsType:                   {Cluster: ClusterByLabel},
	omni.ClusterMachineStatusType:                    {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineTalosVersionType:              {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMachineType:                          {Cluster: ClusterByLabel, Verbs: Read},
	omni.ClusterMetricsType:                          {Verbs: Read},
	omni.ClusterSecretsRotationStatusType:            {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterSecretsType:                          {Cluster: ClusterByID},
	omni.ClusterStatusMetricsType:                    {Verbs: Read},
	omni.ClusterStatusType:                           {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterTaintType:                            {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterType:                                 {Cluster: ClusterByID, Verbs: AllVerbs},
	omni.ClusterUUIDType:                             {Cluster: ClusterByID, Verbs: Read},
	omni.ClusterWorkloadProxyStatusType:              {Cluster: ClusterByID, Verbs: Read},
	omni.ConfigPatchType:                             {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.ControlPlaneStatusType:                      {Cluster: ClusterByLabel, Verbs: Read},
	omni.DiscoveryAffiliateDeleteTaskType:            {Cluster: ClusterByLabel, Verbs: Read},
	omni.EtcdAuditResultType:                         {Cluster: ClusterByID},
	omni.EtcdBackupOverallStatusType:                 {Verbs: Read},
	omni.EtcdBackupS3ConfType:                        {Verbs: AllVerbs},
	omni.EtcdBackupStatusType:                        {Cluster: ClusterByID, Verbs: Read},
	omni.EtcdBackupStoreStatusType:                   {Verbs: Read},
	omni.EtcdBackupType:                              {Cluster: ClusterByLabel, Verbs: Read},
	omni.EtcdManualBackupType:                        {Cluster: ClusterByID, Verbs: AllVerbs},
	omni.ExposedServiceType:                          {Cluster: ClusterByLabel, Verbs: Read},
	omni.ExtensionsConfigurationType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.FeaturesConfigType:                          {Verbs: Read},
	omni.ImagePullRequestType:                        {Cluster: ClusterByLabel, Verbs: Read},
	omni.ImagePullStatusType:                         {Cluster: ClusterByLabel, Verbs: Read},
	omni.ImportedClusterSecretsType:                  {Cluster: ClusterByID, Verbs: Create | Destroy},
	omni.InfraMachineBMCConfigType:                   {Verbs: AllVerbs},
	omni.InfraMachineConfigType:                      {Verbs: AllVerbs},
	omni.InfraProviderCombinedStatusType:             {Verbs: Read},
	omni.InstallationMediaConfigType:                 {Verbs: AllVerbs},
	omni.InstallationMediaType:                       {Verbs: Read},
	omni.KernelArgsStatusType:                        {Cluster: ClusterByLabel, Verbs: Read},
	omni.KernelArgsType:                              {Verbs: AllVerbs},
	omni.KubernetesHealthCheckStatusType:             {Cluster: ClusterByLabel, Verbs: Read},
	omni.KubernetesHealthCheckType:                   {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.KubernetesManifestGroupType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.KubernetesNodeAuditResultType:               {Cluster: ClusterByID, Verbs: Read},
	omni.KubernetesStatusType:                        {Cluster: ClusterByID, Verbs: Read},
	omni.KubernetesUpgradeManifestStatusType:         {Cluster: ClusterByID, Verbs: Read},
	omni.KubernetesUpgradeStatusType:                 {Cluster: ClusterByID, Verbs: Read},
	omni.KubernetesVersionType:                       {Verbs: Read},
	omni.LoadBalancerConfigType:                      {Cluster: ClusterByID, Verbs: Read},
	omni.LoadBalancerStatusType:                      {Cluster: ClusterByID, Verbs: Read},
	omni.MachineClassType:                            {Verbs: AllVerbs},
	omni.MachineConfigDiffType:                       {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineConfigExtractionStatusType:           {Verbs: Read},
	omni.MachineConfigGenOptionsType:                 {Verbs: Read},
	omni.MachineExtensionsStatusType:                 {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineExtensionsType:                       {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineInstallDiskConfigType:                {Verbs: AllVerbs},
	omni.MachineInstallDiskStatusType:                {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineLabelsType:                           {Verbs: AllVerbs},
	omni.MachinePendingUpdatesType:                   {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineRequestSetType:                       {Verbs: AllVerbs},
	omni.MachineSetDestroyStatusType:                 {Verbs: Read},
	omni.MachineSetNodeType:                          {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.MachineSetStatusType:                        {Cluster: ClusterByLabel, Verbs: Read},
	omni.MachineSetType:                              {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.MachineStatusLinkType:                       {Verbs: Read},
	omni.MachineStatusMetricsType:                    {Verbs: Read},
	omni.MachineStatusSnapshotType:                   {Verbs: Read},
	omni.MachineStatusType:                           {Verbs: Read},
	omni.MachineType:                                 {Verbs: Read},
	omni.MachineUpgradeStatusType:                    {Cluster: ClusterByLabel, Verbs: Read},
	omni.MaintenanceConfigStatusType:                 {Verbs: Read},
	omni.NodeForceDestroyRequestType:                 {Cluster: ClusterByLabel, Verbs: AllVerbs},
	omni.NotificationType:                            {Verbs: Read},
	omni.OngoingTaskType:                             {Verbs: Read},
	omni.RedactedClusterMachineConfigType:            {Cluster: ClusterByLabel, Verbs: Read},
	omni.RotateKubernetesCAType:                      {Cluster: ClusterByID, Verbs: AllVerbs},
	omni.RotateTalosCAType:                           {Cluster: ClusterByID, Verbs: AllVerbs},
	omni.SchematicConfigurationType:                  {Cluster: ClusterByLabel, Verbs: Read},
	omni.SecretRotationType:                          {Cluster: ClusterByID},
	omni.TalosExtensionsType:                         {Verbs: Read},
	omni.TalosUpgradeStatusType:                      {Cluster: ClusterByID, Verbs: Read},
	omni.TalosVersionType:                            {Verbs: Read},
	omni.UpgradeRolloutType:                          {Cluster: ClusterByID, Verbs: Read},
	siderolink.APIConfigType:                         {Verbs: Read},
	siderolink.ConnectionParamsType:                  {Verbs: Read},
	siderolink.DefaultJoinTokenType:                  {Verbs: AllVerbs},
	siderolink.GRPCTunnelConfigType:                  {Verbs: AllVerbs},
	siderolink.JoinTokenStatusType:                   {Verbs: Read},
	siderolink.JoinTokenType:                         {Verbs: Read | Update | Destroy},
	siderolink.LinkStatusType:                        {Verbs: Read},
	siderolink.LinkType:                              {Verbs: Read | Destroy},
	siderolink.NodeUniqueTokenStatusType:             {Verbs: Read},
	siderolink.PendingMachineType:                    {Verbs: Read | Destroy},
	system.ResourceLabelsType[*omni.MachineStatus](): {Verbs: Read},
	system.SysVersionType:                            {Verbs: Read},
	virtual.AdvertisedEndpointsType:                  {Verbs: Read},
	virtual.CloudPlatformConfigType:                  {Verbs: Read},
	virtual.ClusterPermissionsType:                   {Verbs: Read},
	virtual.CurrentUserType:                          {Verbs: Read},
	virtual.KubernetesUsageType:                      {Verbs: Read},
	virtual.LabelsCompletionType:                     {Verbs: Read},
	virtual.MetalPlatformConfigType:                  {Verbs: Read},
	virtual.PermissionsType:                          {Verbs: Read},
	virtual.QuirksType:                               {Verbs: Read},
	virtual.SBCConfigType:                            {Verbs: Read},
	virtual.SupportType:                              {Verbs: Read},
	virtual.VersionContractType:                      {Verbs: Read},
}
