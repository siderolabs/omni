// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package management provides client for Omni management API.
package helpers_test

import (
	"errors"
	"testing"

	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/siderolabs/omni/client/pkg/client/helpers"
)

func TestSyncEventConversion(t *testing.T) {
	event := event.Event{
		Type:  event.PruneType,
		Error: errors.New("error!!!!"),
		ObjectID: object.ObjMetadata{
			Namespace: "foo",
			Name:      "bar",
			GroupKind: schema.GroupKind{
				Group: "foo",
				Kind:  "bar",
			},
		},
	}

	protoEvent, err := helpers.ConvertSyncEventToProto(event)
	require.NoError(t, err)
	convertedEvent, err := helpers.DeconvertProtoSyncEvent(protoEvent)
	require.NoError(t, err)

	assert.Equal(t, event, convertedEvent, "expect event to be the same after being converted to a proto representation and back")
}

func TestDiffResultConversion(t *testing.T) {
	result := manifests.DiffResult{
		Object: &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test-obj",
					"namespace": "default",
				},
			},
		},
		Action: manifests.ModifyAction,
		Diff:   "foo",
	}

	resultProto, err := helpers.ConvertDiffResultToProto(result)
	require.NoError(t, err)
	convertedResult, err := helpers.DeconvertProtoDiffResult(resultProto)
	require.NoError(t, err)

	assert.Equal(t, result, convertedResult, "expect result to be the same after being converted to a proto representation and back")
}
