// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import "sigs.k8s.io/cli-utils/pkg/object"

func (x *ClusterKubernetesManifestsStatusSpec) DeleteManifestStatus(group, id string) {
	if x.Groups == nil {
		return
	}

	if gr, ok := x.Groups[group]; ok {
		delete(gr.Manifests, id)

		if len(gr.Manifests) == 0 {
			delete(x.Groups, group)
		}
	}
}

func (x *ClusterKubernetesManifestsStatusSpec) SetManifestStatus(
	group, id string,
	manifest object.ObjMetadata,
	phase ClusterKubernetesManifestsStatusSpec_ManifestStatus_Phase,
) {
	if x.Groups == nil {
		x.Groups = map[string]*ClusterKubernetesManifestsStatusSpec_GroupStatus{}
	}

	gr, ok := x.Groups[group]
	if !ok {
		gr = &ClusterKubernetesManifestsStatusSpec_GroupStatus{
			Manifests: map[string]*ClusterKubernetesManifestsStatusSpec_ManifestStatus{},
		}

		x.Groups[group] = gr
	}

	status := &ClusterKubernetesManifestsStatusSpec_ManifestStatus{
		Phase:     phase,
		Name:      manifest.Name,
		Kind:      manifest.GroupKind.Kind,
		Group:     manifest.GroupKind.Group,
		Namespace: manifest.Namespace,
	}

	gr.Manifests[id] = status
}

func (x *ClusterKubernetesManifestsStatusSpec) UpdateGroups(groups map[string]*KubernetesManifestGroupSpec) {
	x.OutOfSync = 0
	x.Total = 0

	for id, group := range x.Groups {
		group.Phase = ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED

		kmg := groups[id]

		if kmg == nil {
			if len(group.Manifests) == 0 || group.Mode == KubernetesManifestGroupSpec_ONE_TIME {
				delete(x.Groups, id)
			} else {
				group.Phase = ClusterKubernetesManifestsStatusSpec_GroupStatus_DELETING

				x.Total += int32(len(group.Manifests))
				x.OutOfSync += int32(len(group.Manifests))
			}

			continue
		}

		group.Mode = kmg.Mode

		for _, manifest := range group.Manifests {
			x.Total++

			if manifest.Phase != ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED {
				x.OutOfSync++

				group.Phase = ClusterKubernetesManifestsStatusSpec_GroupStatus_PROGRESSING
			}
		}
	}
}

func (x *ClusterKubernetesManifestsStatusSpec) GetManifestPhase(group, id string) ClusterKubernetesManifestsStatusSpec_ManifestStatus_Phase {
	gr, ok := x.Groups[group]

	if !ok {
		return ClusterKubernetesManifestsStatusSpec_ManifestStatus_UNKNOWN
	}

	if m, ok := gr.Manifests[id]; ok {
		return m.Phase
	}

	return ClusterKubernetesManifestsStatusSpec_ManifestStatus_UNKNOWN
}

func (x *ClusterKubernetesManifestsStatusSpec) IsGroupApplied(group string) bool {
	if x.Groups == nil {
		return false
	}

	if s, ok := x.Groups[group]; ok && s.Phase == ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED {
		return true
	}

	return false
}
