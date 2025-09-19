// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// EnsureAllMachinesSupportStrictTokens makes sure that Omni state doesn't have any machines running Talos
// below 1.6 and strict tokens mode is enabled.
func EnsureAllMachinesSupportStrictTokens(ctx context.Context, st state.State) error {
	count, err := getMachinesBelowTalosVersion(ctx, st, siderolink.MinSupportedSecureTokensVersion)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	return fmt.Errorf("detected %s running Talos version below 1.6. 'strict' join token is not supported on the instance\n"+
		"Please upgrade the machines\n"+
		"Or change '--join-tokens-mode' flag to 'legacyAllowed'", pluralize.NewClient().Pluralize("machine", int(count), true))
}

func getMachinesBelowTalosVersion(ctx context.Context, st state.State, belowVersion semver.Version) (int32, error) {
	statuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, st)
	if err != nil {
		return 0, err
	}

	versions := map[string]int32{}

	for status := range statuses.All() {
		if status.TypedSpec().Value.TalosVersion == "" {
			continue
		}

		versions[status.TypedSpec().Value.TalosVersion]++
	}

	if len(versions) == 0 {
		return 0, nil
	}

	var count int32

	for version, c := range versions {
		v, err := semver.ParseTolerant(strings.TrimLeft(version, "v"))
		if err != nil {
			continue
		}

		if v.LT(belowVersion) {
			count += c
		}
	}

	return count, nil
}
