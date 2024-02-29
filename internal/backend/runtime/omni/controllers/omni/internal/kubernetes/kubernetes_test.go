// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/image"
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
			assertVersion(t, v, cfg.Cluster().APIServer().Image())
			assertVersion(t, v, cfg.Cluster().Proxy().Image())
		},
		kubernetes.ControllerManager: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.Cluster().ControllerManager().Image())
		},
		kubernetes.Scheduler: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.Cluster().Scheduler().Image())
		},
		kubernetes.Kubelet: func(t *testing.T, cfg config.Provider, v string) {
			assertVersion(t, v, cfg.Machine().Kubelet().Image())
		},
	}

	for i := 0; i < 10; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			components := append(slices.Clone(kubernetes.AllControlPlaneComponents), kubernetes.Kubelet)
			rand.Shuffle(len(components), func(i, j int) {
				components[i], components[j] = components[j], components[i]
			})

			v1alpha1Cfg := &v1alpha1.Config{}
			cfg := container.NewV1Alpha1(v1alpha1Cfg)

			for _, component := range components {
				component.Patch("1.2.3").Apply(v1alpha1Cfg)
				componentVersionAssertion[component](t, cfg, "1.2.3")
			}
		})
	}
}
