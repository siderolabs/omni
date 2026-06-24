// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	consts "github.com/siderolabs/omni/internal/pkg/constants"
	"github.com/siderolabs/omni/internal/pkg/registry"
)

var (
	// minDiscoveredK8sVersion sets minimum Kubernetes version for building the list of versions.
	minDiscoveredK8sVersion = semver.MustParse(consts.MinDiscoveredKubernetesVersion)
	// minDiscoveredTalosVersion sets minimum Talos version for building the list of versions.
	minDiscoveredTalosVersion = semver.MustParse(consts.MinDiscoveredTalosVersion)
	// minTalosVersion sets minimum Talos version which are not deprecated.
	minTalosVersion = semver.MustParse(constants.MinTalosVersion)
)

// VersionRefreshInterval is the interval in which the kubernetes version is refreshed.
const VersionRefreshInterval = 15 * time.Minute

// VersionsController creates omni.KubernetesVersions and omni.TalosVersions by scanning container registry.
type VersionsController struct {
	imageFactoryClient            *imagefactory.Client
	st                            state.State
	kubernetesRegistry            string
	enableTalosPreReleaseVersions bool
}

// NewVersionsController creates a new VersionsController.
func NewVersionsController(imageFactoryClient *imagefactory.Client, st state.State, enableTalosPreReleaseVersions bool, kubernetesRegistry string) *VersionsController {
	return &VersionsController{
		imageFactoryClient:            imageFactoryClient,
		st:                            st,
		enableTalosPreReleaseVersions: enableTalosPreReleaseVersions,
		kubernetesRegistry:            kubernetesRegistry,
	}
}

// Name implements controller.Controller interface.
func (ctrl *VersionsController) Name() string {
	return "VersionsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VersionsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.TalosVersionType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VersionsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.KubernetesVersionType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: omni.TalosVersionType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VersionsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ticker := time.NewTicker(VersionRefreshInterval)
	defer ticker.Stop()

	if err := ctrl.reconcileAllVersions(ctx, r, logger); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if err := ctrl.reconcileAllVersions(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *VersionsController) reconcileAllVersions(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	k8sVersions, err := ctrl.reconcileKubernetesVersions(ctx, r, logger)
	if err != nil {
		return err
	}

	parsedK8sVersions, err := ctrl.parseK8sVersions(k8sVersions)
	if err != nil {
		return err
	}

	if err = ctrl.reconcileTalosVersions(ctx, r, parsedK8sVersions, logger); err != nil {
		return fmt.Errorf("error reconciling Talos versions: %w", err)
	}

	// After a successful factory request the sniffer transport will have captured the Server header.
	// Update FeaturesConfig with the detected enterprise state. Use direct state access (same path as
	// features.UpdateResources) to avoid controller-ownership conflicts on the unowned resource.
	isEnterprise := ctrl.imageFactoryClient.CachedIsEnterprise()

	featuresConfig := omni.NewFeaturesConfig(omni.FeaturesConfigID)

	if _, err = safe.StateUpdateWithConflicts(ctx, ctrl.st, featuresConfig.Metadata(), func(res *omni.FeaturesConfig) error {
		res.TypedSpec().Value.IsEnterpriseImageFactory = isEnterprise

		return nil
	}); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to update features config with enterprise image factory state: %w", err)
	}

	return nil
}

var allowedPreVersionStrings = map[string]struct{}{
	"alpha": {},
	"beta":  {},
	"rc":    {},
}

func (ctrl *VersionsController) fetchVersionsFromRegistry(ctx context.Context, source string) ([]string, error) {
	return registry.UpgradeCandidates(ctx, source)
}

func (ctrl *VersionsController) fetchTalosVersions(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	return ctrl.imageFactoryClient.Versions(ctx)
}

func (ctrl *VersionsController) getVersionsAfter(versions []string, minVersion semver.Version, includePreReleaseVersions bool) []string {
	res := make([]string, 0, len(versions))

	for _, currentVersion := range versions {
		currentVersion = strings.TrimLeft(currentVersion, "v")

		ver, err := semver.ParseTolerant(currentVersion)
		if err != nil {
			continue
		}

		// skip pre-release versions
		if len(ver.Pre) > 0 {
			if !includePreReleaseVersions {
				continue
			}

			if len(ver.Pre) != 2 {
				continue
			}

			if _, ok := allowedPreVersionStrings[ver.Pre[0].VersionStr]; !ok {
				continue
			}

			if !ver.Pre[1].IsNumeric() {
				continue
			}
		}

		if ver.LT(minVersion) {
			continue
		}

		res = append(res, currentVersion)
	}

	return res
}

func forAllCompatibleVersions(
	talosVersions []string,
	k8sVersions []*compatibility.KubernetesVersion,
	fn func(talosVersion string, k8sVersions []string) error,
) error {
	// Create a map of Kubernetes versions to Kubernetes parsed versions. It should be faster in sort.Slice below.
	versionsToVersions := make(map[string]semver.Version, len(k8sVersions))

	for _, v := range k8sVersions {
		k8sVersion := v.String()

		result, err := semver.Parse(k8sVersion)
		if err != nil {
			return fmt.Errorf("error parsing Kubernetes version %q: %w", k8sVersion, err)
		}

		versionsToVersions[k8sVersion] = result
	}

	// We can preallocate it once, and clone for each WriterModify call - saving some capacity.
	compatibleK8sVersions := make([]string, 0, len(k8sVersions))

	for _, v := range talosVersions {
		parsedTalosVersion, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: v})
		if err != nil {
			return err
		}

		compatibleK8sVersions = compatibleK8sVersions[:0]

		for _, k8sVersion := range k8sVersions {
			if supportErr := k8sVersion.SupportedWith(parsedTalosVersion); supportErr == nil {
				compatibleK8sVersions = append(compatibleK8sVersions, k8sVersion.String())
			}
		}

		if len(compatibleK8sVersions) == 0 {
			continue
		}

		slices.SortFunc(compatibleK8sVersions, func(a, b string) int {
			version1, ok := versionsToVersions[a]
			if !ok {
				return 0
			}

			version2, ok := versionsToVersions[b]
			if !ok {
				return 0
			}

			return version1.Compare(version2)
		})

		if err = fn(v, slices.Clone(compatibleK8sVersions)); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *VersionsController) reconcileTalosVersions(ctx context.Context, r controller.Runtime, k8sVersions []*compatibility.KubernetesVersion, logger *zap.Logger) error {
	tracker := trackResource(r, resources.DefaultNamespace, omni.TalosVersionType)

	allVersions, err := ctrl.fetchTalosVersions(ctx)
	if err != nil {
		return err
	}

	talosVersions := ctrl.getVersionsAfter(allVersions, minDiscoveredTalosVersion, ctrl.enableTalosPreReleaseVersions)

	talosVersions = xslices.FilterInPlace(talosVersions, func(v string) bool {
		return consts.DenylistedTalosVersions.IsAllowed(v)
	})

	upgradeTargets, err := computeUpgradeTargets(talosVersions)
	if err != nil {
		return fmt.Errorf("failed to compute Talos upgrade targets: %w", err)
	}

	latestSupported, err := semver.ParseTolerant(constants.LatestSupportedTalosVersion)
	if err != nil {
		return fmt.Errorf("failed to parse latest supported Talos version %q: %w", constants.LatestSupportedTalosVersion, err)
	}

	err = forAllCompatibleVersions(talosVersions, k8sVersions, func(talosVer string, compatibleK8sVersions []string) error {
		talosVersion := omni.NewTalosVersion(talosVer)
		if writeErr := safe.WriterModify(ctx, r, talosVersion, func(res *omni.TalosVersion) error {
			res.TypedSpec().Value.Version = talosVer
			res.TypedSpec().Value.CompatibleKubernetesVersions = compatibleK8sVersions
			res.TypedSpec().Value.UpgradableTalosVersions = upgradeTargets[talosVer]

			parsed := semver.MustParse(strings.TrimLeft(talosVer, "v"))
			res.TypedSpec().Value.Unsupported = !targetAllowed(parsed, latestSupported.Major, latestSupported.Minor, ctrl.enableTalosPreReleaseVersions)

			stable := parsed
			stable.Pre = nil
			res.TypedSpec().Value.Deprecated = stable.LT(minTalosVersion)

			return nil
		}); writeErr != nil {
			if state.IsPhaseConflictError(writeErr) {
				return nil
			}

			return writeErr
		}

		tracker.keep(talosVersion)

		return nil
	})
	if err != nil {
		return err
	}

	logger.Info("reconciled Talos versions")

	return tracker.cleanup(ctx)
}

// computeUpgradeTargets builds a map from each source Talos version to the full list of versions a machine running that source can be upgraded to per Talos's compatibility matrix.
func computeUpgradeTargets(versions []string) (map[string][]string, error) {
	type candidate struct {
		talos   *compatibility.TalosVersion
		version string
	}

	candidates := make([]candidate, 0, len(versions))
	semverByVersion := make(map[string]semver.Version, len(versions))

	for _, ver := range versions {
		parsed, parseErr := semver.ParseTolerant(ver)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse Talos version %q: %w", ver, parseErr)
		}

		tv, parseErr := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: ver})
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse Talos version %q for compatibility check: %w", ver, parseErr)
		}

		candidates = append(candidates, candidate{
			version: ver,
			talos:   tv,
		})
		semverByVersion[ver] = parsed
	}

	result := make(map[string][]string, len(candidates))

	for _, source := range candidates {
		targets := make([]string, 0, len(candidates))

		for _, target := range candidates {
			if target.version == source.version {
				continue
			}

			if target.talos.UpgradeableFrom(source.talos) != nil {
				continue
			}

			targets = append(targets, target.version)
		}

		slices.SortFunc(targets, func(a, b string) int {
			va := semverByVersion[a]
			vb := semverByVersion[b]

			return va.Compare(vb)
		})

		result[source.version] = targets
	}

	return result, nil
}

// targetAllowed decides whether a Talos version may appear in an upgrade-target list given the global cap on supported versions and the pre-release feature flag.
//
// Examples (latestSupported = 1.13):
//
//	1.12.5              -> allowed (older minor)
//	1.13.0, 1.13.3      -> allowed (within the supported minor)
//	1.14.0              -> rejected (beyond cap, stable release)
//	1.14.0-rc.1         -> allowed only when enablePreReleases is true (next minor, pre-release)
//	1.15.0-alpha.0      -> rejected (skips a minor beyond the cap)
//	2.0.0, 2.0.0-rc.1   -> rejected (new major beyond the cap)
func targetAllowed(target semver.Version, latestSupportedMajor, latestSupportedMinor uint64, enablePreReleases bool) bool {
	if target.Major < latestSupportedMajor {
		return true
	}

	if target.Major == latestSupportedMajor && target.Minor <= latestSupportedMinor {
		return true
	}

	// Beyond the cap. Allow next-minor pre-releases only when the pre-release feature is enabled.
	if !enablePreReleases {
		return false
	}

	if target.Major != latestSupportedMajor || target.Minor != latestSupportedMinor+1 {
		return false
	}

	return len(target.Pre) > 0
}

func (ctrl *VersionsController) reconcileKubernetesVersions(ctx context.Context, r controller.Runtime, logger *zap.Logger) ([]string, error) {
	tracker := trackResource(r, resources.DefaultNamespace, omni.KubernetesVersionType)

	allVersions, err := ctrl.fetchVersionsFromRegistry(ctx, ctrl.kubernetesRegistry)
	if err != nil {
		return nil, err
	}

	versions := ctrl.getVersionsAfter(allVersions, minDiscoveredK8sVersion, false)

	for _, v := range versions {
		k8sVersion := omni.NewKubernetesVersion(v)

		if err = safe.WriterModify(ctx, r, k8sVersion, func(res *omni.KubernetesVersion) error {
			res.TypedSpec().Value.Version = v

			return nil
		}); err != nil {
			return nil, err
		}

		tracker.keep(k8sVersion)
	}

	logger.Info("reconciled Kubernetes versions")

	err = tracker.cleanup(ctx)
	if err != nil {
		return nil, err
	}

	return versions, nil
}

func (ctrl *VersionsController) parseK8sVersions(versions []string) ([]*compatibility.KubernetesVersion, error) {
	res := make([]*compatibility.KubernetesVersion, 0, len(versions))

	for _, k8sVersion := range versions {
		parsedVersion, err := compatibility.ParseKubernetesVersion(k8sVersion)
		if err != nil {
			return nil, err
		}

		res = append(res, parsedVersion)
	}

	return res, nil
}
