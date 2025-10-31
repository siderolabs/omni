// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations_test

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/maps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

//go:embed testdata/export/cluster-template.yaml
var clusterTemplate string

//go:embed testdata/export/cluster-resources.yaml
var clusterResources []byte

type resources struct {
	clusters        map[string]*omni.Cluster
	machineSets     map[string]*omni.MachineSet
	machineSetNodes map[string]*omni.MachineSetNode
	configPatches   map[string]*omni.ConfigPatch
}

func TestExport(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	st := buildState(ctx, t)

	// export a template from resources created via the UI
	exportedTemplate := assertTemplate(ctx, t, st)

	// sync the exported template back into the state and assert that the state is unchanged
	assertSync(ctx, t, st, exportedTemplate)

	// export a template one more time and assert that it's still the same as the first exported template
	assertTemplate(ctx, t, st)
}

// assertSync asserts that the state is unchanged after syncing back the exported template.
func assertSync(ctx context.Context, t *testing.T, st state.State, exportedTemplate string) {
	var sb strings.Builder

	resourcesBeforeSync := readResources(ctx, t, st)

	err := operations.SyncTemplate(ctx, strings.NewReader(exportedTemplate), &sb, st, operations.SyncOptions{})
	require.NoError(t, err)

	resourcesAfterSync := readResources(ctx, t, st)

	assert.ElementsMatch(t, maps.Keys(resourcesBeforeSync.clusters), maps.Keys(resourcesAfterSync.clusters))
	assert.ElementsMatch(t, maps.Keys(resourcesBeforeSync.machineSets), maps.Keys(resourcesAfterSync.machineSets))
	assert.ElementsMatch(t, maps.Keys(resourcesBeforeSync.machineSetNodes), maps.Keys(resourcesAfterSync.machineSetNodes))
	assert.ElementsMatch(t, maps.Keys(resourcesBeforeSync.configPatches), maps.Keys(resourcesAfterSync.configPatches))

	// we expect everything other than config patches to be completely unchanged
	assertVersionsUnchanged(t, resourcesBeforeSync.clusters, resourcesAfterSync.clusters)
	assertVersionsUnchanged(t, resourcesBeforeSync.machineSets, resourcesAfterSync.machineSets)
	assertVersionsUnchanged(t, resourcesBeforeSync.machineSetNodes, resourcesAfterSync.machineSetNodes)

	// config patches might be updated due to discrepancies between how the cluster templates and the frontend generate them, e.g.:
	// they might differ in the indentation/comments of the patch data, so we do a yaml-equality check on data instead of byte-equality.
	assertConfigPatches(t, resourcesBeforeSync.configPatches, resourcesAfterSync.configPatches)
}

func assertVersionsUnchanged[T resource.Resource](t *testing.T, before map[resource.ID]T, after map[resource.ID]T) {
	for _, beforeResource := range before {
		afterResource := after[beforeResource.Metadata().ID()]

		assert.Equal(t, beforeResource.Metadata().Version(), afterResource.Metadata().Version(), "resource version changed for resource: %q", beforeResource.Metadata())
	}
}

func assertConfigPatches(t *testing.T, before map[string]*omni.ConfigPatch, after map[string]*omni.ConfigPatch) {
	getPatchData := func(patch *omni.ConfigPatch) string {
		buffer, err := patch.TypedSpec().Value.GetUncompressedData()
		require.NoError(t, err)

		defer buffer.Free()

		return string(buffer.Data())
	}

	for _, beforeConfigPatch := range before {
		afterConfigPatch := after[beforeConfigPatch.Metadata().ID()]

		if beforeConfigPatch.Metadata().Version().Equal(afterConfigPatch.Metadata().Version()) {
			continue
		}

		assert.Equal(t, beforeConfigPatch.Metadata().Labels(), afterConfigPatch.Metadata().Labels(), "labels changed for resource: %q", beforeConfigPatch.Metadata())
		assert.Equal(t, beforeConfigPatch.Metadata().Annotations(), afterConfigPatch.Metadata().Annotations(), "annotations changed for resource: %q", beforeConfigPatch.Metadata())

		beforeData := getPatchData(beforeConfigPatch)
		afterData := getPatchData(afterConfigPatch)

		assert.YAMLEq(t, beforeData, afterData,
			"config patch data changed for resource: %q", beforeConfigPatch.Metadata())
	}
}

func readResources(ctx context.Context, t *testing.T, st state.State) resources {
	clusterList, err := safe.StateListAll[*omni.Cluster](ctx, st)
	require.NoError(t, err)

	machineSetList, err := safe.StateListAll[*omni.MachineSet](ctx, st)
	require.NoError(t, err)

	machineSetNodeList, err := safe.StateListAll[*omni.MachineSetNode](ctx, st)
	require.NoError(t, err)

	configPatchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, st)
	require.NoError(t, err)

	return resources{
		clusters:        listToMap(clusterList),
		machineSets:     listToMap(machineSetList),
		machineSetNodes: listToMap(machineSetNodeList),
		configPatches:   listToMap(configPatchList),
	}
}

func assertTemplate(ctx context.Context, t *testing.T, st state.State) string {
	var sb strings.Builder

	modelList, err := operations.ExportTemplate(ctx, st, "export-test", &sb)
	require.NoError(t, err)

	assertAllFieldsSet(t, modelList)

	assert.Equal(t, clusterTemplate, sb.String())

	return clusterTemplate
}

// assertAllFieldsSet asserts that all fields of the models are set at least once.
// This is to ensure that the exporter is handling all fields of the models as new fields are added.
func assertAllFieldsSet(t *testing.T, modelList models.List) {
	machineSetTypeName := reflect.TypeFor[models.MachineSet]().String()
	modelFieldToIsZero := make(map[string]bool)

	for _, model := range modelList {
		modelValue := reflect.ValueOf(model).Elem()
		modelType := modelValue.Type()
		remainingMachineSetFieldCount := 0

		for _, visibleField := range reflect.VisibleFields(modelType) {
			modelField := modelType.String() + "." + visibleField.Name

			// if the field belongs to the MachineSet struct, we do not want to check twice for ControlPlane and Workers types
			if visibleField.Anonymous && visibleField.Type.String() == machineSetTypeName {
				remainingMachineSetFieldCount = len(reflect.VisibleFields(visibleField.Type))
			} else {
				remainingMachineSetFieldCount--
			}

			if remainingMachineSetFieldCount >= 0 { // the field belongs to the MachineSet Anonymous field
				modelField = machineSetTypeName + "." + visibleField.Name
			}

			if isZero, ok := modelFieldToIsZero[modelField]; !ok || isZero {
				modelFieldToIsZero[modelField] = modelValue.FieldByName(visibleField.Name).IsZero()
			}
		}
	}

	for modelField, isZero := range modelFieldToIsZero {
		assert.False(t, isZero, "field %q was never set, is the field handled by the exporter?", modelField)
	}
}

func buildState(ctx context.Context, t *testing.T) state.State {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	dec := yaml.NewDecoder(bytes.NewReader(clusterResources))

	for {
		var res protobuf.YAMLResource

		err := dec.Decode(&res)
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		require.NoError(t, st.Create(ctx, res.Resource(), state.WithCreateOwner(res.Resource().Metadata().Owner())))
	}

	return st
}

func listToMap[T resource.Resource](list safe.List[T]) map[resource.ID]T {
	result := make(map[resource.ID]T, list.Len())

	for value := range list.All() {
		result[value.Metadata().ID()] = value
	}

	return result
}
