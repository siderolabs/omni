// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rbac

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// Resource family constants.
const (
	FamilyClusters = "clusters"
	FamilyMachines = "machines"
	FamilyUsers    = "users"
	FamilyAuth     = "auth"
	FamilySystem   = "system"
)

// ResourceFamilyConfig defines the family and allowed COSI verbs for a resource type.
type ResourceFamilyConfig struct {
	Family       string
	AllowedVerbs map[state.Verb]struct{}
}

// Verb sets for common patterns.
var (
	readOnly = newVerbSet(state.Get, state.List, state.Watch)
	fullCRUD = newVerbSet(state.Get, state.List, state.Watch, state.Create, state.Update, state.Destroy)
	noCreate = newVerbSet(state.Get, state.List, state.Watch, state.Update, state.Destroy)
)

func newVerbSet(verbs ...state.Verb) map[state.Verb]struct{} {
	s := make(map[state.Verb]struct{}, len(verbs))
	for _, v := range verbs {
		s[v] = struct{}{}
	}

	return s
}

// Registry maps each externally accessible resource type to its family and allowed verbs.
//
// Types not in the registry are denied to all external users.
// AuthConfigType is handled as a standalone exception (public, no auth required) and is not in any family.
//
//nolint:gochecknoglobals
var Registry = map[resource.Type]ResourceFamilyConfig{
	// =========================================================================
	// clusters family — all cluster-scoped resources
	// =========================================================================

	// full CRUD
	omni.ClusterType:                 {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.MachineSetType:              {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.MachineSetNodeType:          {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.ConfigPatchType:             {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.EtcdManualBackupType:        {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.ImportedClusterSecretsType:  {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.RotateTalosCAType:          {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.RotateKubernetesCAType:     {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.ExtensionsConfigurationType: {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.KernelArgsType:             {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.NodeForceDestroyRequestType: {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.KubernetesManifestGroupType: {Family: FamilyClusters, AllowedVerbs: fullCRUD},
	omni.InfraMachineConfigType:      {Family: FamilyClusters, AllowedVerbs: fullCRUD},

	// read-only (controller-managed)
	omni.ClusterBootstrapStatusType:           {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterConfigVersionType:             {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterDestroyStatusType:             {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterEndpointType:                  {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterKubernetesNodesType:           {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterWorkloadProxyStatusType:       {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterTaintType:                     {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterUUIDType:                      {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterStatusType:                    {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterDiagnosticsType:               {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.EtcdBackupStatusType:                 {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterSecretsRotationStatusType:     {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterKubernetesManifestsStatusType: {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineIdentityType:           {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineType:                   {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineConfigPatchesType:      {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineConfigStatusType:       {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineTalosVersionType:       {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineStatusType:             {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ClusterMachineRequestStatusType:      {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ExposedServiceType:                   {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ImagePullRequestType:                 {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ImagePullStatusType:                  {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineSetStatusType:                 {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineSetDestroyStatusType:          {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineRequestSetStatusType:          {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.EtcdBackupType:                       {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.SchematicConfigurationType:           {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineUpgradeStatusType:             {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.KernelArgsStatusType:                 {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineExtensionsStatusType:          {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineExtensionsType:                {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.DiscoveryAffiliateDeleteTaskType:     {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.RedactedClusterMachineConfigType:     {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachineConfigDiffType:                {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.MachinePendingUpdatesType:            {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.ControlPlaneStatusType:               {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.KubernetesNodeAuditResultType:        {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.KubernetesStatusType:                 {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.KubernetesUpgradeManifestStatusType:  {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.KubernetesUpgradeStatusType:          {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.LoadBalancerConfigType:               {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.LoadBalancerStatusType:               {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.TalosUpgradeStatusType:               {Family: FamilyClusters, AllowedVerbs: readOnly},
	omni.UpgradeRolloutType:                   {Family: FamilyClusters, AllowedVerbs: readOnly},

	// =========================================================================
	// machines family — unbound machines and machine metadata
	// =========================================================================

	// full CRUD
	omni.MachineLabelsType:           {Family: FamilyMachines, AllowedVerbs: fullCRUD},
	omni.MachineClassType:            {Family: FamilyMachines, AllowedVerbs: fullCRUD},
	omni.MachineRequestSetType:       {Family: FamilyMachines, AllowedVerbs: fullCRUD},
	omni.InstallationMediaConfigType: {Family: FamilyMachines, AllowedVerbs: fullCRUD},
	siderolink.GRPCTunnelConfigType:  {Family: FamilyMachines, AllowedVerbs: fullCRUD},

	// no create (update+destroy only)
	siderolink.LinkType:           {Family: FamilyMachines, AllowedVerbs: noCreate},
	siderolink.PendingMachineType: {Family: FamilyMachines, AllowedVerbs: noCreate},

	// read-only (controller-managed)
	omni.MachineType:                     {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.MachineStatusType:               {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.MachineStatusSnapshotType:       {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.MachineStatusLinkType:           {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.MachineConfigGenOptionsType:     {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.MaintenanceConfigStatusType:     {Family: FamilyMachines, AllowedVerbs: readOnly},
	omni.InfraProviderCombinedStatusType: {Family: FamilyMachines, AllowedVerbs: readOnly},
	siderolink.LinkStatusType:            {Family: FamilyMachines, AllowedVerbs: readOnly},
	siderolink.ConnectionParamsType:      {Family: FamilyMachines, AllowedVerbs: readOnly},
	siderolink.APIConfigType:             {Family: FamilyMachines, AllowedVerbs: readOnly},

	// =========================================================================
	// users family — user and service account management
	// =========================================================================

	// full CRUD
	authres.SAMLLabelRuleType:  {Family: FamilyUsers, AllowedVerbs: fullCRUD},
	omni.EtcdBackupS3ConfType: {Family: FamilyUsers, AllowedVerbs: fullCRUD},

	// read-only (mutations via ManagementService only)
	authres.IdentityType:             {Family: FamilyUsers, AllowedVerbs: readOnly},
	authres.IdentityLastActiveType:   {Family: FamilyUsers, AllowedVerbs: readOnly},
	authres.IdentityStatusType:       {Family: FamilyUsers, AllowedVerbs: readOnly},
	authres.UserType:                 {Family: FamilyUsers, AllowedVerbs: readOnly},
	authres.ServiceAccountStatusType: {Family: FamilyUsers, AllowedVerbs: readOnly},

	// =========================================================================
	// auth family — RBAC and access policy management
	// =========================================================================

	// full CRUD
	authres.AccessPolicyType:        {Family: FamilyAuth, AllowedVerbs: fullCRUD},
	authres.RoleType:                {Family: FamilyAuth, AllowedVerbs: fullCRUD},
	authres.RoleBindingType:         {Family: FamilyAuth, AllowedVerbs: fullCRUD},
	siderolink.DefaultJoinTokenType: {Family: FamilyAuth, AllowedVerbs: fullCRUD},

	// no create (must use management.CreateJoinToken API)
	siderolink.JoinTokenType: {Family: FamilyAuth, AllowedVerbs: noCreate},

	// =========================================================================
	// system family — always readable by any authenticated user
	// =========================================================================

	meta.NamespaceType:                               {Family: FamilySystem, AllowedVerbs: readOnly},
	meta.ResourceDefinitionType:                      {Family: FamilySystem, AllowedVerbs: readOnly},
	system.SysVersionType:                            {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.FeaturesConfigType:                          {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.OngoingTaskType:                             {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.NotificationType:                            {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.ClusterMetricsType:                          {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.ClusterStatusMetricsType:                    {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.MachineStatusMetricsType:                    {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.TalosVersionType:                            {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.KubernetesVersionType:                       {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.TalosExtensionsType:                         {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.InstallationMediaType:                       {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.SchematicType:                               {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.EtcdBackupOverallStatusType:                 {Family: FamilySystem, AllowedVerbs: readOnly},
	omni.EtcdBackupStoreStatusType:                   {Family: FamilySystem, AllowedVerbs: readOnly},
	siderolink.JoinTokenStatusType:                   {Family: FamilySystem, AllowedVerbs: readOnly},
	siderolink.NodeUniqueTokenStatusType:             {Family: FamilySystem, AllowedVerbs: readOnly},
	system.ResourceLabelsType[*omni.MachineStatus](): {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.AdvertisedEndpointsType:                  {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.CurrentUserType:                          {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.PermissionsType:                          {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.ClusterPermissionsType:                   {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.SBCConfigType:                            {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.CloudPlatformConfigType:                  {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.MetalPlatformConfigType:                  {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.LabelsCompletionType:                     {Family: FamilySystem, AllowedVerbs: readOnly},
	virtual.KubernetesUsageType:                      {Family: FamilySystem, AllowedVerbs: readOnly},
}
