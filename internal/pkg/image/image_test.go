// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/image"
)

func TestGetTag(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "valid image",
			input: "docker.io/autonomy/installer:latest",
			want:  "latest",
		},
		{
			name:  "image with version",
			input: "registry.k8s.io/kube-apiserver:v1.26.1",
			want:  "v1.26.1",
		},
		{
			name:    "image with digest",
			input:   "registry.k8s.io/kube-apiserver@sha256:99e1ed9fbc8a8d36a70f148f25130c02e0e366875249906be0bcb2c2d9df0c26",
			wantErr: `image reference "registry.k8s.io/kube-apiserver@sha256:99e1ed9fbc8a8d36a70f148f25130c02e0e366875249906be0bcb2c2d9df0c26" doesn't have a tag`,
		},
		{
			name:    "iamge without tag",
			input:   "docker.io/autonomy/installer",
			wantErr: `image reference "docker.io/autonomy/installer" doesn't have a tag`,
		},
		{
			name:    "invalid image",
			input:   "docker.io/autonomy/installer:latest:latest",
			wantErr: "invalid reference format",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := image.GetTag(test.input)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
			} else {
				require.NoError(t, err)

				assert.Equal(t, test.want, got)
			}
		})
	}
}
