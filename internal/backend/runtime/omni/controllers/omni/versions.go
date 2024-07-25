// Copyright (c) 2024 Sidero Labs, Inc.
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

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	consts "github.com/siderolabs/omni/internal/pkg/constants"
	"github.com/siderolabs/omni/internal/pkg/registry"
)

var (
	// minK8sVersion sets minimum Kubernetes version for building the list of versions.
	minK8sVersion = semver.MustParse(consts.MinKubernetesVersion)
	// minDiscoveredTalosVersion sets minimum Talos version for building the list of versions.
	minDiscoveredTalosVersion = semver.MustParse(consts.MinDiscoveredTalosVersion)
	// minTalosVersion sets minimum Talos version which are not deprecated.
	minTalosVersion = semver.MustParse(consts.MinTalosVersion)
)

// VersionRefreshInterval is the interval in which the kubernetes version is refreshed.
const VersionRefreshInterval = 15 * time.Minute

// VersionsController creates omni.KubernetesVersions and omni.TalosVersions by scanning container registry.
type VersionsController struct{}

// Name implements controller.Controller interface.
func (ctrl *VersionsController) Name() string {
	return "VersionsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VersionsController) Inputs() []controller.Input {
	return []controller.Input{}
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

	return nil
}

var allowedPreVersionStrings = map[string]struct{}{
	"alpha": {},
	"beta":  {},
}

func (ctrl *VersionsController) getVersionsAfter(ctx context.Context, source string, min semver.Version, includePreReleaseVersions bool) ([]string, error) {
	versions, err := registry.UpgradeCandidates(ctx, source)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(versions))

	for _, currentVersion := range versions {
		currentVersion = strings.TrimLeft(currentVersion, "v")

		var ver semver.Version

		ver, err = semver.ParseTolerant(currentVersion)
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

		if ver.LT(min) {
			continue
		}

		res = append(res, currentVersion)
	}

	return res, nil
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

	talosVersions, err := ctrl.getVersionsAfter(ctx, config.Config.TalosRegistry, minDiscoveredTalosVersion, config.Config.EnableTalosPreReleaseVersions)
	if err != nil {
		return err
	}

	talosVersions = xslices.FilterInPlace(talosVersions, func(v string) bool {
		_, denylisted := consts.DenylistedTalosVersions[v]

		return !denylisted
	})

	err = forAllCompatibleVersions(talosVersions, k8sVersions, func(talosVer string, compatibleK8sVersions []string) error {
		talosVersion := omni.NewTalosVersion(resources.DefaultNamespace, talosVer)
		if writeErr := safe.WriterModify(ctx, r, talosVersion, func(res *omni.TalosVersion) error {
			res.TypedSpec().Value.Version = talosVer
			res.TypedSpec().Value.CompatibleKubernetesVersions = compatibleK8sVersions

			v := semver.MustParse(strings.TrimLeft(talosVer, "v"))
			v.Pre = nil

			res.TypedSpec().Value.Deprecated = v.LT(minTalosVersion)

			return nil
		}); writeErr != nil {
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

func (ctrl *VersionsController) reconcileKubernetesVersions(ctx context.Context, r controller.Runtime, logger *zap.Logger) ([]string, error) {
	tracker := trackResource(r, resources.DefaultNamespace, omni.KubernetesVersionType)

	versions, err := ctrl.getVersionsAfter(ctx, config.Config.KubernetesRegistry, minK8sVersion, false)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		k8sVersion := omni.NewKubernetesVersion(resources.DefaultNamespace, v)

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
