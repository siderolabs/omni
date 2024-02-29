// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package helpers_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

func TestUpdateInputsVersions(t *testing.T) {
	out := omni.NewCluster("default", "test")

	in := []resource.Resource{omni.NewMachine("default", "test1"), omni.NewMachine("default", "test2")}

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ := out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "a7a451e614fc3b4a7241798235001fea271c7ad5493c392f0a012104379bdb89", v)

	assert.False(t, helpers.UpdateInputsVersions(out, in...))

	in = append(in, omni.NewClusterMachine("default", "cm1"))

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ = out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "df4af53c3caf7ae4c0446bcf8b854ed3f5740a47eab0e5151f1962a4a4d52f6f", v)
}
