// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/template"
)

//go:embed testdata/cluster1.yaml
var cluster1 []byte

//go:embed testdata/cluster2.yaml
var cluster2 []byte

//go:embed testdata/cluster3.yaml
var cluster3 []byte

//go:embed testdata/cluster-valid-bootstrapspec.yaml
var clusterValidBootstrapSpec []byte

//go:embed testdata/cluster-bad-yaml1.yaml
var clusterBadYAML1 []byte

//go:embed testdata/cluster-bad-yaml2.yaml
var clusterBadYAML2 []byte

//go:embed testdata/cluster-bad-yaml3.yaml
var clusterBadYAML3 []byte

//go:embed testdata/cluster-invalid1.yaml
var clusterInvalid1 []byte

//go:embed testdata/cluster-invalid2.yaml
var clusterInvalid2 []byte

//go:embed testdata/cluster-invalid3.yaml
var clusterInvalid3 []byte

//go:embed testdata/cluster-invalid4.yaml
var clusterInvalid4 []byte

//go:embed testdata/cluster-invalid-bootstrapspec.yaml
var clusterInvalidBootstrapSpec []byte

//go:embed testdata/cluster1-resources.yaml
var cluster1Resources []byte

//go:embed testdata/cluster2-resources.yaml
var cluster2Resources []byte

//go:embed testdata/cluster3-resources.yaml
var cluster3Resources []byte

//go:embed testdata/cluster-valid-bootstrapspec-resources.yaml
var clusterValidBootstrapSpecResources []byte

func TestLoad(t *testing.T) {
	for _, tt := range []struct { //nolint:govet
		name          string
		data          []byte
		expectedError string
	}{
		{
			name: "cluster1",
			data: cluster1,
		},
		{
			name: "cluster2",
			data: cluster2,
		},
		{
			name:          "clusterBadYAML1",
			data:          clusterBadYAML1,
			expectedError: "error decoding document at line 1:1: yaml: unmarshal errors:\n  line 7: field containerd not found in type models.Cluster",
		},
		{
			name:          "clusterBadYAML2",
			data:          clusterBadYAML2,
			expectedError: "error in document at line 1:1: unknown model kind \"FunnyCluster\"",
		},
		{
			name:          "clusterBadYAML3",
			data:          clusterBadYAML3,
			expectedError: "error in document at line 1:1: kind field not found",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := template.Load(bytes.NewReader(tt.data))
			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Chdir("testdata")

	for _, tt := range []struct { //nolint:govet
		name          string
		data          []byte
		expectedError string
	}{
		{
			name: "cluster1",
			data: cluster1,
		},
		{
			name: "cluster2",
			data: cluster2,
		},
		{
			name: "cluster3",
			data: cluster3,
		},
		{
			name: "clusterInvalid1",
			data: clusterInvalid1,
			expectedError: `5 errors occurred:
	* error validating cluster "my first cluster": 5 errors occurred:
	* name should only contain letters, digits, dashes and underscores
	* error validating Kubernetes version: 1 error occurred:
	* version should be in semver format: Invalid character(s) found in major number "N"


	* error validating Talos version: 1 error occurred:
	* version should be in semver format: Invalid character(s) found in patch number "0gamma.0"


	* patch "does-not-exist.yaml" is invalid: 1 error occurred:
	* failed to access "does-not-exist.yaml": open does-not-exist.yaml: no such file or directory


	* patch "" is invalid: 1 error occurred:
	* either name or idOverride is required for inline patches




	* controlplane is invalid: 4 errors occurred:
	* updateStrategy is not allowed in the controlplane
	* deleteStrategy is not allowed in the controlplane
	* patch "patches/invalid.yaml" is invalid: 1 error occurred:
	* failed to validate patch "patches/invalid.yaml": missing kind


	* patch "kubespan-enabled" is invalid: 1 error occurred:
	* failed to validate inline patch "kubespan-enabled": unknown keys found during decoding:
machine:
    network:
        kubespan:
            running: true





	* machines [4aed1106-6f44-4be9-9796-d4b5b0b5b0b0] are used in both controlplane and workers
	* machine "630d882a-51a8-48b3-ae00-90c5b0b5b0b0" is not used in controlplane or workers
	* machine "430d882a-51a8-48b3-ae00-90c5b0b5b0b0" is locked and used in controlplane

`,
		},
		{
			name: "clusterInvalid2",
			data: clusterInvalid2,
			expectedError: `3 errors occurred:
	* error validating cluster "": 4 errors occurred:
	* name is required
	* error validating Kubernetes version: 1 error occurred:
	* version is required


	* error validating Talos version: 1 error occurred:
	* version is required


	* patch "" is invalid: 1 error occurred:
	* path or inline is required




	* controlplane is invalid: 1 error occurred:
	* patch "patches/prohibited.yaml" is invalid: 1 error occurred:
	* failed to validate patch "patches/prohibited.yaml": 1 error occurred:
	* overriding "machine.token" is not allowed in the config patch






	* template should contain 1 controlplane, got 2

`,
		},
		{
			name: "clusterInvalid3",
			data: clusterInvalid3,
			expectedError: `3 errors occurred:
	* controlplane is invalid: 1 error occurred:
	* custom name is not allowed in the controlplane


	* duplicate workers with name "additional-1"
	* machine "b1ed45d8-4e79-4a07-a29a-b1b075843d41" is used in multiple workers: ["additional-1" "additional-2"]

`,
		},
		{
			name: "clusterInvalid4",
			data: clusterInvalid4,
			expectedError: `2 errors occurred:
	* error validating cluster "my-first-cluster": 1 error occurred:
	* disk encryption is supported only for Talos version >= 1.5.0


	* workers is invalid: 1 error occurred:
	* machine set can not have both machines and machine class defined`,
		},
		{
			name: "clusterInvalidBootstrapSpec",
			data: clusterInvalidBootstrapSpec,
			expectedError: `2 errors occurred:
	* controlplane is invalid: 2 errors occurred:
	* clusterUUID is required in bootstrapSpec
	* snapshot is required in bootstrapSpec


	* workers is invalid: 1 error occurred:
	* bootstrapSpec is not allowed in workers`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			templ, err := template.Load(bytes.NewReader(tt.data))
			require.NoError(t, err)

			err = templ.Validate()
			if tt.expectedError == "" {
				require.NoError(t, err)

				return
			}

			printMode := false
			if printMode {
				fmt.Printf("ACTUAL ERROR:\n%s\n", err.Error())

				return
			}

			require.Equal(t, strings.TrimSpace(tt.expectedError), strings.TrimSpace(err.Error()), err.Error())
		})
	}
}

func TestTranslate(t *testing.T) {
	previousCompressionConfig := specs.GetCompressionConfig()

	// disable resource compression for the test
	compressionConfig, err := compression.BuildConfig(false, false, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		specs.SetCompressionConfig(previousCompressionConfig)
	})

	specs.SetCompressionConfig(compressionConfig)

	t.Chdir("testdata")

	for _, tt := range []struct {
		name     string
		template []byte
		expected []byte
	}{
		{
			name:     "cluster1",
			template: cluster1,
			expected: cluster1Resources,
		},
		{
			name:     "cluster2",
			template: cluster2,
			expected: cluster2Resources,
		},
		{
			name:     "cluster3",
			template: cluster3,
			expected: cluster3Resources,
		},
		{
			name:     "clusterValidBootstrapSpec",
			template: clusterValidBootstrapSpec,
			expected: clusterValidBootstrapSpecResources,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			templ, err := template.Load(bytes.NewReader(tt.template))
			require.NoError(t, err)

			require.NoError(t, templ.Validate())

			resources, err := templ.Translate()
			require.NoError(t, err)

			var actual bytes.Buffer

			enc := yaml.NewEncoder(&actual)

			for _, r := range resources {
				// zero timestamps for reproducibility
				r.Metadata().SetCreated(time.Time{})
				r.Metadata().SetUpdated(time.Time{})

				m, err := resource.MarshalYAML(r)
				require.NoError(t, err)

				require.NoError(t, enc.Encode(m))
			}

			printMode := false
			if printMode {
				fmt.Printf("ACTUAL:\n%s\n", actual.String())

				return
			}

			require.Equal(t, string(tt.expected), actual.String())
		})
	}
}

func TestSync(t *testing.T) {
	t.Chdir("testdata")

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx := t.Context()

	templ1, err := template.Load(bytes.NewReader(cluster1))
	require.NoError(t, err)

	sync1, err := templ1.Sync(ctx, st)
	require.NoError(t, err)

	assert.Equal(t, [][]resource.Resource{nil, nil}, sync1.Destroy)
	assert.Empty(t, sync1.Update)
	assert.Len(t, sync1.Create, 12)

	for _, r := range sync1.Create {
		require.NoError(t, st.Create(ctx, r))
	}

	templ2, err := template.Load(bytes.NewReader(cluster2))
	require.NoError(t, err)

	sync2, err := templ2.Sync(ctx, st)
	require.NoError(t, err)

	assert.Equal(t, [][]string{
		{
			"MachineSetNodes.omni.sidero.dev(default/4aed1106-6f44-4be9-9796-d4b5b0b5b0b0)",
		},
		{
			"ConfigPatches.omni.sidero.dev(default/400-my-first-cluster-control-planes-patches/my-cp-patch.yaml)",
			"ConfigPatches.omni.sidero.dev(default/401-my-first-cluster-control-planes-kubespan-enabled)",
		},
	}, xslices.Map(sync2.Destroy, func(x []resource.Resource) []string { return xslices.Map(x, resource.String) }))

	assert.Equal(t, []string{
		"Clusters.omni.sidero.dev(default/my-first-cluster)",
		"ConfigPatches.omni.sidero.dev(default/000-cm-430d882a-51a8-48b3-ae00-90c5b0b5b0b0-install-disk)",
		"MachineSetNodes.omni.sidero.dev(default/430d882a-51a8-48b3-ab00-d4b5b0b5b0b0)",
	}, xslices.Map(sync2.Update, func(u template.UpdateChange) string { return resource.String(u.New) }))

	assert.Equal(t, []string{
		"ConfigPatches.omni.sidero.dev(default/400-my-first-cluster-control-planes-kubespan-enabled)",
	}, xslices.Map(sync2.Create, resource.String))
}

func TestDelete(t *testing.T) {
	t.Chdir("testdata")

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx := t.Context()

	templ1, err := template.Load(bytes.NewReader(cluster1))
	require.NoError(t, err)

	sync1, err := templ1.Sync(ctx, st)
	require.NoError(t, err)

	for _, r := range sync1.Create {
		require.NoError(t, st.Create(ctx, r))
	}

	templ2, err := template.Load(bytes.NewReader(cluster2))
	require.NoError(t, err)

	syncDelete, err := templ2.Delete(ctx, st)
	require.NoError(t, err)

	assert.Empty(t, syncDelete.Create)
	assert.Empty(t, syncDelete.Update)
	assert.Len(t, syncDelete.Destroy, 2)
	assert.Len(t, syncDelete.Destroy[0], 0)
	assert.Len(t, syncDelete.Destroy[1], 1)
}
