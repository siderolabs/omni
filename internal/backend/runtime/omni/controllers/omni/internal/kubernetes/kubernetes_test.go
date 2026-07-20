// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/image"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
)

func TestComponentLess(t *testing.T) {
	t.Parallel()

	assert.True(t, kubernetes.APIServer.Less(kubernetes.ControllerManager))
	assert.True(t, kubernetes.ControllerManager.Less(kubernetes.Scheduler))
	assert.True(t, kubernetes.Scheduler.Less(kubernetes.Kubelet))
}

func TestComponentPatch(t *testing.T) {
	t.Parallel()

	assertVersion := func(t *testing.T, v string, ref string) {
		tag, err := image.GetTag(ref)

		require.NoError(t, err)
		assert.Equal(t, v, strings.TrimLeft(tag, "v"))
	}

	componentVersionAssertion := map[kubernetes.Component]func(*testing.T, config.Provider, string){
		kubernetes.APIServer: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.K8sAPIServerConfig().Image())
			assertVersion(t, v, cfg.K8sProxyConfig().Image())
		},
		kubernetes.ControllerManager: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.K8sControllerManagerConfig().Image())
		},
		kubernetes.Scheduler: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.K8sSchedulerConfig().Image())
		},
		kubernetes.Kubelet: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.Machine().Kubelet().Image())
		},
	}

	for _, vc := range []*config.VersionContract{config.TalosVersion1_13, config.TalosVersion1_14} {
		t.Run(vc.String(), func(t *testing.T) {
			t.Parallel()

			for i := range 10 {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					t.Parallel()

					components := append(slices.Clone(kubernetes.AllControlPlaneComponents), kubernetes.Kubelet)
					rand.Shuffle(len(components), func(i, j int) {
						components[i], components[j] = components[j], components[i]
					})

					in, err := generate.NewInput("component-patch-test", "https://127.0.0.1/", "1.34.0", generate.WithVersionContract(vc))
					require.NoError(t, err)

					cfg, err := in.Config(machine.TypeControlPlane)
					require.NoError(t, err)

					for _, component := range components {
						patcher, err := component.Patch(vc, "1.2.3")
						require.NoError(t, err)

						patch, err := configpatcher.LoadPatch(patcher.Patch)
						require.NoError(t, err)

						patched, err := configpatcher.Apply(configpatcher.WithConfig(cfg), []configpatcher.Patch{patch})
						require.NoError(t, err)

						cfg, err = patched.Config()
						require.NoError(t, err)

						componentVersionAssertion[component](t, cfg, "1.2.3")
					}
				})
			}
		})
	}
}
