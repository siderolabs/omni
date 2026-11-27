// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package virtual provides a virtual state implementation.
package virtual

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	machineryConfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/stripe/stripe-go/v87"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	stateerrors "github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/errors"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/factory/configs"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const supportEnabledCacheTTL = 5 * time.Minute

// State is a virtual state implementation which provides virtual resources.
// Virtual resources are not stored in the backend, but are generated on the fly.
// They also do not behave like regular resources; for example, their content might differ, according to the caller/context.
type State struct {
	supportEnabledCachedAt   time.Time
	PrimaryState             state.State
	computed                 map[resource.Type]*Computed
	stripeClient             *stripe.Client
	apiURL                   string
	stripeSubscriptionItemID string
	registryConfig           config.Registries
	supportConfig            config.Support
	supportEnabledMu         sync.Mutex
	supportEnabled           bool
}

// NewState creates new virtual state instance.
func NewState(state state.State, stripeClient *stripe.Client, apiURL, stripeSubscriptionItemID string, registryConfig config.Registries, supportConfig config.Support) *State {
	return &State{
		PrimaryState:             state,
		computed:                 map[resource.Type]*Computed{},
		apiURL:                   apiURL,
		stripeClient:             stripeClient,
		stripeSubscriptionItemID: stripeSubscriptionItemID,
		supportConfig:            supportConfig,
		registryConfig:           registryConfig,
	}
}

// RunComputed starts a new computed producer on the virtual state
// which runs until the context is canceled.
func (v *State) RunComputed(ctx context.Context, t resource.Type, factory ProducerFactory, resolveID ProducerIDTransformer, logger *zap.Logger) {
	st := NewComputed(t, factory, resolveID, time.Minute, logger, true)

	v.computed[t] = st

	st.Run(ctx)
}

// Get implements state.CoreState.
//
//nolint:cyclop
func (v *State) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if len(opts) > 0 {
		return nil, stateerrors.ErrUnsupported(errors.New("no get options are supported"))
	}

	if err := v.validateKind(ptr); err != nil {
		return nil, err
	}

	computed, ok := v.computed[ptr.Type()]
	if ok {
		return computed.Get(ctx, ptr, opts...)
	}

	switch ptr.Type() {
	case virtual.CurrentUserType:
		if ptr.ID() != virtual.CurrentUserID {
			return nil, stateerrors.ErrNotFound(ptr)
		}

		return v.currentUser(ctx)
	case virtual.MetalPlatformConfigType:
		res, err := configs.GetMetalPlatformConfig(ptr)
		if err != nil {
			return nil, err
		}

		return res, nil
	case virtual.SBCConfigType:
		res, err := configs.GetSBCConfig(ptr)
		if err != nil {
			return nil, err
		}

		return res, nil
	case virtual.CloudPlatformConfigType:
		res, err := configs.GetCloudPlatformConfig(ptr)
		if err != nil {
			return nil, err
		}

		return res, nil
	case virtual.PermissionsType:
		return v.permissions(ctx)
	case virtual.ClusterPermissionsType:
		return v.clusterPermissions(ctx, ptr)
	case virtual.LabelsCompletionType:
		return v.labelsCompletion(ctx, ptr)
	case virtual.AdvertisedEndpointsType:
		return v.advertisedEndpoints(ctx, ptr)
	case virtual.SupportType:
		if ptr.ID() != virtual.SupportID {
			return nil, stateerrors.ErrNotFound(ptr)
		}

		return v.support(ctx)
	case virtual.QuirksType:
		return v.quirks(ctx, ptr)
	case virtual.VersionContractType:
		return v.versionContract(ctx, ptr)
	default:
		return nil, stateerrors.ErrUnsupported(fmt.Errorf("unsupported resource type for get %q", ptr.Type()))
	}
}

// List implements state.CoreState.
func (v *State) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if len(opts) > 0 {
		var options state.ListOptions

		for _, o := range opts {
			o(&options)
		}

		if len(options.LabelQueries) != 0 {
			return resource.List{}, stateerrors.ErrUnsupported(errors.New("label queries are not supported"))
		}

		if options.IDQuery.Regexp != nil {
			return resource.List{}, stateerrors.ErrUnsupported(errors.New("id query is not supported"))
		}
	}

	if err := v.validateKind(kind); err != nil {
		return resource.List{}, err
	}

	switch kind.Type() {
	case virtual.MetalPlatformConfigType:
		return configs.ListMetalPlatformConfigs(), nil
	case virtual.SBCConfigType:
		return configs.ListSBCConfigs(), nil
	case virtual.CloudPlatformConfigType:
		return configs.ListCloudPlatformConfigs(), nil
	case virtual.CurrentUserType:
		user, err := v.currentUser(ctx)
		if err != nil {
			return resource.List{}, err
		}

		return resource.List{Items: []resource.Resource{user}}, nil
	case virtual.PermissionsType:
		permissions, err := v.permissions(ctx)
		if err != nil {
			return resource.List{}, err
		}

		return resource.List{Items: []resource.Resource{permissions}}, nil
	case virtual.QuirksType:
		return v.listQuirks(ctx)
	default:
		return resource.List{}, stateerrors.ErrUnsupported(fmt.Errorf("unsupported resource type for list %q", kind.Type()))
	}
}

// Create implements state.CoreState.
func (v *State) Create(_ context.Context, _ resource.Resource, _ ...state.CreateOption) error {
	return stateerrors.ErrUnsupported(errors.New("create is not supported"))
}

// Update implements state.CoreState.
func (v *State) Update(_ context.Context, _ resource.Resource, _ ...state.UpdateOption) error {
	return stateerrors.ErrUnsupported(errors.New("update is not supported"))
}

// Destroy implements state.CoreState.
func (v *State) Destroy(_ context.Context, _ resource.Pointer, _ ...state.DestroyOption) error {
	return stateerrors.ErrUnsupported(errors.New("destroy is not supported"))
}

// Watch implements state.CoreState.
func (v *State) Watch(ctx context.Context, ptr resource.Pointer, c chan<- state.Event, opts ...state.WatchOption) error {
	computed, ok := v.computed[ptr.Type()]
	if ok {
		return computed.Watch(ctx, ptr, c, opts...)
	}

	return stateerrors.ErrUnsupported(fmt.Errorf("unsupported resource type for watch %q", ptr.Type()))
}

// WatchKind implements state.CoreState.
func (v *State) WatchKind(_ context.Context, ptr resource.Kind, _ chan<- state.Event, _ ...state.WatchKindOption) error {
	return stateerrors.ErrUnsupported(fmt.Errorf("unsupported resource type for watch kind %q", ptr.Type()))
}

// WatchKindAggregated implements state.CoreState.
func (v *State) WatchKindAggregated(_ context.Context, _ resource.Kind, _ chan<- []state.Event, _ ...state.WatchKindOption) error {
	return stateerrors.ErrUnsupported(errors.New("watch kind aggregated is not supported"))
}

func (v *State) validateKind(kind resource.Kind) error {
	if kind.Namespace() != resources.VirtualNamespace {
		return stateerrors.ErrUnsupported(fmt.Errorf("unsupported namespace: %s", kind.Namespace()))
	}

	return nil
}

func (v *State) currentUser(ctx context.Context) (*virtual.CurrentUser, error) {
	identityVal, _ := ctxstore.Value[auth.IdentityContextKey](ctx)

	userRole := role.None

	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		userRole = val.Role
	}

	user := virtual.NewCurrentUser()

	user.TypedSpec().Value.Identity = identityVal.Identity
	user.TypedSpec().Value.Role = string(userRole)

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	user.Metadata().SetVersion(version)

	identity, err := safe.ReaderGetByID[*authres.Identity](ctx, v.PrimaryState, identityVal.Identity)
	if err != nil {
		if state.IsNotFoundError(err) {
			return user, nil
		}

		return nil, err
	}

	user.TypedSpec().Value.UserId = identity.TypedSpec().Value.UserId

	return user, nil
}

// permissions returns what the UI can offer to the caller outside of a single cluster.
//
// Each flag is the answer of the access check which the action behind it goes through.
// A flag of a page which lists everything asks for everything: the list is refused to a caller with access to some clusters only.
func (v *State) permissions(ctx context.Context) (*virtual.Permissions, error) {
	subject, err := grants.SubjectWithAllGrants(ctx, v.PrimaryState)
	if err != nil {
		return nil, err
	}

	// on the machines in no cluster and on the resources of the instance
	can := func(group authz.ResourceGroup, verb authz.Verbs) bool {
		return authz.CheckResourceGroup(subject, group, verb, authz.Target{}) == nil
	}

	// a flag about managing something is for the pages which create, change and delete it
	canManage := func(group authz.ResourceGroup) bool {
		return can(group, authz.Create) && can(group, authz.Update) && can(group, authz.Destroy)
	}

	permissions := virtual.NewPermissions()
	spec := permissions.TypedSpec().Value

	spec.CanReadClusters = authz.CheckResourceGroup(subject, authz.Clusters, authz.Read, authz.Target{AllClusters: true}) == nil
	// the name of a new cluster is not known here, so access to create a cluster with some name is enough
	spec.CanCreateClusters = authz.CheckResourceGroupInAnyScope(subject, authz.Clusters, authz.Create) == nil

	spec.CanReadMachines = can(authz.Machines, authz.Read)
	spec.CanReadMachineLogs = can(authz.Machines, authz.Read)
	spec.CanRemoveMachines = can(authz.Machines, authz.Destroy)
	spec.CanAccessMaintenanceNodes = authz.CheckOperation(subject, authz.MachineMaintenance, authz.Target{}) == nil

	spec.CanReadMachineConfigPatches = can(authz.MachineManagement, authz.Read)
	spec.CanManageMachineConfigPatches = canManage(authz.MachineManagement)
	spec.CanReadJoinTokens = can(authz.MachineManagement, authz.Read)
	spec.CanManageJoinTokens = canManage(authz.MachineManagement)
	spec.CanReadInstallationMedia = can(authz.MachineManagement, authz.Read)
	spec.CanManageInstallationMedia = canManage(authz.MachineManagement)

	spec.CanManageUsers = canManage(authz.AccessControl)
	spec.CanManageBackupStore = canManage(authz.Credentials)
	// the same check as the ReadAuditLog RPC
	spec.CanReadAuditLog = authz.CheckOperation(subject, authz.AuditLog, authz.Target{}) == nil

	// the Controllers and DependencyGraph RPCs still check the preset of the user
	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		spec.CanAccessInternals = val.Role.Check(role.Admin) == nil
	}

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	permissions.Metadata().SetVersion(version)

	return permissions, nil
}

// clusterPermissions returns what the UI can offer to the caller on a cluster, see permissions.
func (v *State) clusterPermissions(ctx context.Context, ptr resource.Pointer) (*virtual.ClusterPermissions, error) {
	subject, err := grants.SubjectWithAllGrants(ctx, v.PrimaryState)
	if err != nil {
		return nil, err
	}

	cluster := authz.Target{Cluster: ptr.ID()}

	can := func(group authz.ResourceGroup, verb authz.Verbs) bool {
		return authz.CheckResourceGroup(subject, group, verb, cluster) == nil
	}

	clusterPermissions := virtual.NewClusterPermissions(ptr.ID())
	spec := clusterPermissions.TypedSpec().Value

	// the same check as the Kubeconfig and Talosconfig RPCs: the configs contain no credentials, so every user can get them
	spec.CanDownloadKubeconfig = subject.Authenticated && !subject.Provider
	spec.CanDownloadTalosconfig = subject.Authenticated && !subject.Provider

	spec.CanReadConfigPatches = can(authz.Clusters, authz.Read)
	spec.CanReadMachineConfig = can(authz.Clusters, authz.Read)
	spec.CanReadMachinePendingUpdates = can(authz.Clusters, authz.Read)
	spec.CanReadKubernetesManifests = can(authz.Clusters, authz.Read)

	spec.CanAddMachines = can(authz.Clusters, authz.Create)
	spec.CanRemoveMachines = can(authz.Clusters, authz.Destroy)
	spec.CanUpdateKubernetes = can(authz.Clusters, authz.Update)
	spec.CanUpdateTalos = can(authz.Clusters, authz.Update)
	// the page creates, changes and deletes config patches
	spec.CanManageConfigPatches = can(authz.Clusters, authz.Create) && can(authz.Clusters, authz.Update) && can(authz.Clusters, authz.Destroy)
	spec.CanManageClusterFeatures = can(authz.Clusters, authz.Update)
	spec.CanManageMachineConfig = can(authz.Clusters, authz.Update)
	// the same check as the KubernetesSyncManifests RPC
	spec.CanSyncKubernetesManifests = can(authz.Clusters, authz.Update)

	// The flag is for three buttons: a reboot goes through the Talos API, a shutdown and a power on through the machine power RPCs.
	spec.CanRebootMachines = can(authz.Talos, authz.Update) && authz.CheckOperation(subject, authz.MachinePower, cluster) == nil

	// the same check as the GetSupportBundle RPC
	spec.CanDownloadSupportBundle = authz.CheckOperation(subject, authz.ClusterDiagnostics, cluster) == nil

	// The kernel args are a setting of the machine, which stays with it when it leaves the cluster.
	// So access to the cluster is not enough for them.
	canMachineSettings := func(verb authz.Verbs) bool {
		return authz.CheckResourceGroup(subject, authz.MachineManagement, verb, authz.Target{}) == nil
	}

	spec.CanReadKernelArgs = canMachineSettings(authz.Read)
	spec.CanManageKernelArgs = canMachineSettings(authz.Create) && canMachineSettings(authz.Update) && canMachineSettings(authz.Destroy)

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	clusterPermissions.Metadata().SetVersion(version)

	return clusterPermissions, nil
}

func (v *State) labelsCompletion(ctx context.Context, ptr resource.Pointer) (*virtual.LabelsCompletion, error) {
	md := resource.NewMetadata(resources.DefaultNamespace, ptr.ID(), "", resource.VersionUndefined)

	list, err := v.PrimaryState.List(ctx, md)
	if err != nil {
		return nil, err
	}

	labels := map[string]*specs.LabelsCompletionSpec_Values{}

	for _, res := range list.Items {
		for label, value := range res.Metadata().Labels().Raw() {
			if _, ok := labels[label]; !ok {
				labels[label] = &specs.LabelsCompletionSpec_Values{}
			}

			if !slices.Contains(labels[label].Items, value) {
				labels[label].Items = append(labels[label].Items, value)
			}
		}
	}

	completion := virtual.NewLabelsCompletion(ptr.ID())
	completion.TypedSpec().Value.Items = labels

	return completion, nil
}

func (v *State) support(ctx context.Context) (*virtual.Support, error) {
	res := virtual.NewSupport()

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	res.Metadata().SetVersion(version)

	oh := v.supportConfig.OfficeHours
	res.TypedSpec().Value.OfficeHours = &specs.OfficeHoursConfig{
		Summary:     oh.GetSummary(),
		Description: oh.GetDescription(),
		DtstartTzid: oh.GetDtstartTzid(),
		DtendTzid:   oh.GetDtendTzid(),
		DtstartUtc:  oh.GetDtstartUtc(),
		DtendUtc:    oh.GetDtendUtc(),
		Rrule:       oh.GetRrule(),
		MeetUrl:     oh.GetMeetUrl(),
	}

	if v.stripeClient == nil || v.stripeSubscriptionItemID == "" {
		return res, nil
	}

	v.supportEnabledMu.Lock()
	defer v.supportEnabledMu.Unlock()

	if time.Since(v.supportEnabledCachedAt) > supportEnabledCacheTTL {
		enabled, err := fetchSupportEnabled(ctx, v.stripeClient, v.stripeSubscriptionItemID, v.supportConfig.SupportEligibleProducts)
		if err != nil {
			return nil, err
		}

		v.supportEnabled = enabled
		v.supportEnabledCachedAt = time.Now()
	}

	res.TypedSpec().Value.SupportEnabled = v.supportEnabled

	return res, nil
}

func (v *State) listQuirks(ctx context.Context) (resource.List, error) {
	list, err := v.PrimaryState.List(ctx, resource.NewMetadata(resources.DefaultNamespace, omniresources.TalosVersionType, "", resource.VersionUndefined))
	if err != nil {
		return resource.List{}, err
	}

	items := make([]resource.Resource, 0, len(list.Items))

	for _, item := range list.Items {
		q, err := v.quirks(ctx, item.Metadata())
		if err != nil {
			return resource.List{}, err
		}

		items = append(items, q)
	}

	return resource.List{Items: items}, nil
}

func (v *State) quirks(_ context.Context, ptr resource.Pointer) (*virtual.Quirks, error) {
	res := virtual.NewQuirks(ptr.ID())

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	q := quirks.New(ptr.ID())

	res.Metadata().SetVersion(version)
	res.TypedSpec().Value = &specs.QuirksSpec{
		SupportsUnifiedInstaller: q.SupportsUnifiedInstaller(),
		SupportsFactoryTalosctl:  q.SupportsFactoryTalosctlDownload(),
		SupportsEmbeddedConfig:   q.SupportsEmbeddedConfig(),
	}

	return res, nil
}

func (v *State) versionContract(_ context.Context, ptr resource.Pointer) (*virtual.VersionContract, error) {
	res := virtual.NewVersionContract(ptr.ID())

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	var vc *machineryConfig.VersionContract

	vc, err = machineryConfig.ParseContractFromVersion(ptr.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract from version: %w", err)
	}

	res.Metadata().SetVersion(version)
	res.TypedSpec().Value = &specs.VersionContractSpec{
		KubeSpanMultidocConfig:            vc.KubeSpanMultidocConfig(),
		MultidocKubernetesConfigSupported: vc.MultidocKubernetesConfigSupported(),
		MultidocNetworkConfigSupported:    vc.MultidocNetworkConfigSupported(),
		DiscoveryServiceMultidocConfig:    vc.DiscoveryServiceMultidocConfig(),
	}

	return res, nil
}

func fetchSupportEnabled(ctx context.Context, stripeClient *stripe.Client, stripeSubscriptionItemID string, eligibleProducts []string) (bool, error) {
	params := &stripe.SubscriptionItemRetrieveParams{}
	params.AddExpand("price.product")

	item, err := stripeClient.V1SubscriptionItems.Retrieve(ctx, stripeSubscriptionItemID, params)
	if err != nil {
		return false, err
	}

	if item.Price == nil || item.Price.Product == nil {
		return false, nil
	}

	productName := strings.ToLower(item.Price.Product.Name)

	for _, eligible := range eligibleProducts {
		if strings.Contains(productName, eligible) {
			return true, nil
		}
	}

	return false, nil
}

func (v *State) advertisedEndpoints(_ context.Context, ptr resource.Pointer) (*virtual.AdvertisedEndpoints, error) {
	if ptr.ID() != virtual.AdvertisedEndpointsID {
		return nil, fmt.Errorf("invalid endpoints id %q", ptr.ID())
	}

	res := virtual.NewAdvertisedEndpoints()

	res.TypedSpec().Value.GrpcApiUrl = v.apiURL

	return res, nil
}
