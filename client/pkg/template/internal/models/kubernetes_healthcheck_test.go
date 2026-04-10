// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/siderolabs/gen/pair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

const testJobManifest = `apiVersion: batch/v1
kind: Job
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: check
          image: alpine
          command: ["true"]
`

func TestKubernetesHealthCheck_Validate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct { //nolint:govet
		name string
		hc   models.KubernetesHealthCheck
		err  string
	}{
		{
			name: "valid inline job",
			hc: models.KubernetesHealthCheck{
				Name: "noop",
				Job:  models.NewInlineContentBytes([]byte(testJobManifest)),
			},
		},
		{
			name: "missing name and idOverride",
			hc: models.KubernetesHealthCheck{
				Job: models.NewInlineContentBytes([]byte(testJobManifest)),
			},
			err: "either name or idOverride is required",
		},
		{
			name: "missing job and file",
			hc: models.KubernetesHealthCheck{
				Name: "noop",
			},
			err: "job or file is required",
		},
		{
			name: "both job and file",
			hc: models.KubernetesHealthCheck{
				Name: "noop",
				Job:  models.NewInlineContentBytes([]byte(testJobManifest)),
				File: "noop.yaml",
			},
			err: "mutually exclusive",
		},
		{
			name: "file does not exist",
			hc: models.KubernetesHealthCheck{
				Name: "noop",
				File: "does-not-exist.yaml",
			},
			err: "failed to access",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.hc.Validate(models.ValidateOptions{})
			if tt.err == "" {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestKubernetesHealthCheck_Validate_File(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "job.yaml"), []byte(testJobManifest), 0o600))

	hc := models.KubernetesHealthCheck{
		Name: "noop",
		File: "job.yaml",
	}

	require.NoError(t, hc.Validate(models.ValidateOptions{FileContext: models.FileContext{Dir: dir}}))
}

func TestKubernetesHealthCheckList_DuplicateDetection(t *testing.T) {
	t.Parallel()

	list := models.KubernetesHealthCheckList{
		{
			Name: "dup",
			Job:  models.NewInlineContentBytes([]byte(testJobManifest)),
		},
		{
			Name: "dup",
			Job:  models.NewInlineContentBytes([]byte(testJobManifest)),
		},
	}

	err := list.Validate(models.ValidateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate healthcheck "dup"`)
}

func TestKubernetesHealthCheck_Translate(t *testing.T) {
	t.Parallel()

	hc := models.KubernetesHealthCheck{
		Name: "nodes-ready",
		Job:  models.NewInlineContentBytes([]byte(testJobManifest)),
		Descriptors: models.Descriptors{
			Labels:      map[string]string{"custom-label": "value"},
			Annotations: map[string]string{"description": "checks all nodes are ready"},
		},
	}

	res, err := hc.Translate(models.TranslateContext{}, "cluster-foo", pair.MakePair(omni.LabelCluster, "foo"))
	require.NoError(t, err)

	assert.Equal(t, "cluster-foo-nodes-ready", res.Metadata().ID())

	cluster, ok := res.Metadata().Labels().Get(omni.LabelCluster)
	assert.True(t, ok)
	assert.Equal(t, "foo", cluster)

	custom, ok := res.Metadata().Labels().Get("custom-label")
	assert.True(t, ok)
	assert.Equal(t, "value", custom)

	desc, ok := res.Metadata().Annotations().Get("description")
	assert.True(t, ok)
	assert.Equal(t, "checks all nodes are ready", desc)

	assert.Equal(t, testJobManifest, res.TypedSpec().Value.Job)
}

func TestKubernetesHealthCheck_Translate_IDOverride(t *testing.T) {
	t.Parallel()

	hc := models.KubernetesHealthCheck{
		IDOverride: "exact-id",
		Job:        models.NewInlineContentBytes([]byte(testJobManifest)),
	}

	res, err := hc.Translate(models.TranslateContext{}, "ignored-prefix")
	require.NoError(t, err)
	assert.Equal(t, "exact-id", res.Metadata().ID())
}

func TestKubernetesHealthCheck_Translate_File(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "job.yaml")
	require.NoError(t, os.WriteFile(path, []byte(testJobManifest), 0o600))

	hc := models.KubernetesHealthCheck{
		Name: "nodes-ready",
		File: "job.yaml",
	}

	ctx := models.TranslateContext{FileContext: models.FileContext{Dir: dir}}

	require.NoError(t, hc.Validate(models.ValidateOptions{FileContext: ctx.FileContext}))

	res, err := hc.Translate(ctx, "cluster-foo", pair.MakePair(omni.LabelCluster, "foo"))
	require.NoError(t, err)

	assert.Equal(t, testJobManifest, res.TypedSpec().Value.Job)
}
