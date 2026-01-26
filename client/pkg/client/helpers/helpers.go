// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package helpers provides various utilities used in Omni.
package helpers

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/siderolabs/omni/client/api/omni/management"
)

func ConvertSyncEventToProto(e event.Event) (*management.KubernetesBootstrapManifestSyncResponse, error) {
	var respType management.KubernetesBootstrapManifestSyncResponse_ResponseType

	switch e.Type {
	case event.ApplyType:
		respType = management.KubernetesBootstrapManifestSyncResponse_APPLY
	case event.RolloutType:
		respType = management.KubernetesBootstrapManifestSyncResponse_ROLLOUT
	case event.WaitType:
		respType = management.KubernetesBootstrapManifestSyncResponse_WAIT
	case event.PruneType:
		respType = management.KubernetesBootstrapManifestSyncResponse_PRUNE
	default:
		return &management.KubernetesBootstrapManifestSyncResponse{}, fmt.Errorf("unknown kubernetes manifest sync event: %v", e.Type)
	}

	resp := management.KubernetesBootstrapManifestSyncResponse{
		Type: respType,
		ObjectId: &management.KubernetesBootstrapManifestSyncResponse_ObjectMetadata{
			Namespace: e.ObjectID.Namespace,
			Name:      e.ObjectID.Name,
			GroupKind: e.ObjectID.GroupKind.Kind,
			GroupName: e.ObjectID.GroupKind.Group,
		},
	}

	if e.Error != nil {
		resp.Error = e.Error.Error()
	}

	return &resp, nil
}

func DeconvertProtoSyncEvent(r *management.KubernetesBootstrapManifestSyncResponse) (event.Event, error) {
	var t event.Type

	switch r.Type {
	case management.KubernetesBootstrapManifestSyncResponse_APPLY:
		t = event.ApplyType
	case management.KubernetesBootstrapManifestSyncResponse_ROLLOUT:
		t = event.RolloutType
	case management.KubernetesBootstrapManifestSyncResponse_WAIT:
		t = event.WaitType
	case management.KubernetesBootstrapManifestSyncResponse_PRUNE:
		t = event.PruneType
	default:
		return event.Event{}, fmt.Errorf("unknown proto response type: %v", r.Type)
	}

	var err error
	if r.Error != "" {
		err = errors.New(r.Error)
	}

	// Assuming object.ObjMetadata and schema.GroupKind are used by event.Event
	return event.Event{
		Type:  t,
		Error: err,
		ObjectID: object.ObjMetadata{
			Namespace: r.ObjectId.Namespace,
			Name:      r.ObjectId.Name,
			GroupKind: schema.GroupKind{
				Group: r.ObjectId.GroupName,
				Kind:  r.ObjectId.GroupKind,
			},
		},
	}, nil
}

func ConvertDiffResultToProto(r manifests.DiffResult) (*management.KubernetesBootstrapManifestDiffResponse, error) {
	json, err := r.Object.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubernetes object: %s", manifests.FormatObjectPath(r.Object))
	}

	diffItem := &management.KubernetesBootstrapManifestDiffResponse{
		Object: json,
		Diff:   r.Diff,
	}

	switch r.Action {
	case manifests.CreateAction:
		diffItem.Action = management.KubernetesBootstrapManifestDiffResponse_CREATE
	case manifests.PruneAction:
		diffItem.Action = management.KubernetesBootstrapManifestDiffResponse_PRUNE
	case manifests.ModifyAction:
		diffItem.Action = management.KubernetesBootstrapManifestDiffResponse_MODIFY
	default:
		return &management.KubernetesBootstrapManifestDiffResponse{}, fmt.Errorf("unknown diff action: %v", r.Action)
	}

	return diffItem, nil
}

func DeconvertProtoDiffResult(r *management.KubernetesBootstrapManifestDiffResponse) (manifests.DiffResult, error) {
	var action manifests.DiffAction

	switch r.Action {
	case management.KubernetesBootstrapManifestDiffResponse_CREATE:
		action = manifests.CreateAction
	case management.KubernetesBootstrapManifestDiffResponse_PRUNE:
		action = manifests.PruneAction
	case management.KubernetesBootstrapManifestDiffResponse_MODIFY:
		action = manifests.ModifyAction
	default:
		return manifests.DiffResult{}, fmt.Errorf("unknown diff action: %v", r.Action)
	}

	obj := &unstructured.Unstructured{}
	if len(r.Object) > 0 {
		if err := obj.UnmarshalJSON(r.Object); err != nil {
			return manifests.DiffResult{}, fmt.Errorf("failed to unmarshal kubernetes object json: %w", err)
		}
	}

	return manifests.DiffResult{
		Object: obj,
		Diff:   r.Diff,
		Action: action,
	}, nil
}
