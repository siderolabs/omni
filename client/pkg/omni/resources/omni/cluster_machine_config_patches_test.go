// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const (
	newLineAndText = `
    master: 2
	workers: 3`

	justText = `master: 1
text: "test"`
)

func TestClusterMachineConfigPatchesSpecW_marshal(t *testing.T) {
	original := omni.NewClusterMachineConfigPatches("default", "1")

	err := original.TypedSpec().Value.SetUncompressedPatches([]string{newLineAndText, justText})
	require.NoError(t, err)

	out := must.Value(yaml.Marshal(must.Value(resource.MarshalYAML(original))(t)))(t)

	var dest protobuf.YAMLResource

	err = yaml.Unmarshal(out, &dest)
	require.NoError(t, err)

	fmt.Println(string(out))

	if !resource.Equal(original, dest.Resource()) {
		t.Log("original -->", string(must.Value(yaml.Marshal(original.Spec()))(t)))
		t.Log("result  -->", string(must.Value(yaml.Marshal(dest.Resource().Spec()))(t)))
		t.FailNow()
	}
}

func ExampleClusterMachineSpec_marshal() {
	original := omni.NewClusterMachineConfigPatches("default", "1")

	if err := original.TypedSpec().Value.SetUncompressedPatches([]string{newLineAndText, justText}); err != nil {
		panic(err)
	}

	current := time.Date(2022, 12, 9, 0, 0, 0, 0, time.UTC)
	original.Metadata().SetCreated(current)
	original.Metadata().SetUpdated(current)

	wrap, err := resource.MarshalYAML(original)
	if err != nil {
		panic(err)
	}

	out, err := yaml.Marshal(wrap)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))

	// Output:
	// metadata:
	//     namespace: default
	//     type: ClusterMachineConfigPatches.omni.sidero.dev
	//     id: 1
	//     version: undefined
	//     owner:
	//     phase: running
	//     created: 2022-12-09T00:00:00Z
	//     updated: 2022-12-09T00:00:00Z
	// spec:
	//     patches:
	//         - "\n    master: 2\n\tworkers: 3"
	//         - |-
	//           master: 1
	//           text: "test"
}
