// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type {
  ClusterKubernetesManifestsStatusSpec,
  ClusterKubernetesManifestsStatusSpecGroupStatus,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
  KubernetesManifestGroupSpecMode,
} from '@/api/omni/specs/omni.pb'
import { ClusterKubernetesManifestsStatusType, DefaultNamespace } from '@/api/resources'

import ClusterManifestsStatus from './ClusterManifestsStatus.vue'

const clusterId = 'talos-default'

const meta: Meta<typeof ClusterManifestsStatus> = {
  component: ClusterManifestsStatus,
  args: {
    cluster: clusterId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

const k8sKinds = [
  { kind: 'Deployment', group: 'apps', namespaced: true },
  { kind: 'DaemonSet', group: 'apps', namespaced: true },
  { kind: 'StatefulSet', group: 'apps', namespaced: true },
  { kind: 'ConfigMap', group: '', namespaced: true },
  { kind: 'Secret', group: '', namespaced: true },
  { kind: 'Service', group: '', namespaced: true },
  { kind: 'ServiceAccount', group: '', namespaced: true },
  { kind: 'ClusterRole', group: 'rbac.authorization.k8s.io', namespaced: false },
  { kind: 'ClusterRoleBinding', group: 'rbac.authorization.k8s.io', namespaced: false },
  { kind: 'Role', group: 'rbac.authorization.k8s.io', namespaced: true },
  { kind: 'RoleBinding', group: 'rbac.authorization.k8s.io', namespaced: true },
  { kind: 'CustomResourceDefinition', group: 'apiextensions.k8s.io', namespaced: false },
  { kind: 'NetworkPolicy', group: 'networking.k8s.io', namespaced: true },
]

const k8sNamespaces = ['kube-system', 'default', 'monitoring', 'cert-manager', 'ingress-nginx']

const makeGroup = (
  phase: ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  manifestPhases: ClusterKubernetesManifestsStatusSpecManifestStatusPhase[],
  mode: KubernetesManifestGroupSpecMode,
): ClusterKubernetesManifestsStatusSpecGroupStatus => ({
  phase,
  mode,
  manifests: Object.fromEntries(
    manifestPhases.map((phase) => {
      const resource = faker.helpers.arrayElement(k8sKinds)
      const name = faker.helpers
        .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
        .toLowerCase()
      const ns = resource.namespaced ? faker.helpers.arrayElement(k8sNamespaces) : undefined

      return [
        ns ? `${resource.kind}/${ns}/${name}` : `${resource.kind}/${name}`,
        {
          phase,
          kind: resource.kind,
          name,
          namespace: ns,
          group: resource.group || undefined,
        },
      ] as const
    }),
  ),
})

const makeSpec = (
  seed: number,
  groups: Record<string, ReturnType<typeof makeGroup>>,
  lastError?: string,
): ClusterKubernetesManifestsStatusSpec => {
  faker.seed(seed)
  const allManifests = Object.values(groups).flatMap((g) => Object.values(g.manifests ?? {}))
  const outOfSync = allManifests.filter(
    (m) => m.phase !== ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
  ).length

  return {
    groups,
    total: allManifests.length,
    out_of_sync: outOfSync || undefined,
    last_error: lastError,
  }
}

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterKubernetesManifestsStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterKubernetesManifestsStatusType,
            id: clusterId,
          },
          initialResources: () => {
            faker.seed(4)

            const allPhases = [
              ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
              ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
              ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING,
            ]
            const groupPhases = [
              ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED,
              ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING,
              ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PENDING,
              ClusterKubernetesManifestsStatusSpecGroupStatusPhase.DELETING,
            ]
            const groups = Object.fromEntries(
              faker.helpers.multiple(
                () => {
                  const name = faker.helpers
                    .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
                    .toLowerCase()
                  const groupPhase = faker.helpers.arrayElement(groupPhases)
                  const manifestCount = faker.number.int({ min: 2, max: 12 })
                  const manifests = faker.helpers.multiple(
                    () => faker.helpers.arrayElement(allPhases),
                    { count: manifestCount },
                  )

                  return [
                    name,
                    makeGroup(
                      groupPhase,
                      manifests,
                      faker.helpers.enumValue(KubernetesManifestGroupSpecMode),
                    ),
                  ]
                },
                { count: 12 },
              ),
            )

            return [
              {
                metadata: {
                  namespace: DefaultNamespace,
                  type: ClusterKubernetesManifestsStatusType,
                  id: clusterId,
                },
                spec: makeSpec(4, groups),
              },
            ]
          },
        }).handler,
      ],
    },
  },
}

export const WithError: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterKubernetesManifestsStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterKubernetesManifestsStatusType,
            id: clusterId,
          },
          initialResources: () => {
            faker.seed(3)

            return [
              {
                metadata: {
                  namespace: DefaultNamespace,
                  type: ClusterKubernetesManifestsStatusType,
                  id: clusterId,
                },
                spec: makeSpec(
                  3,
                  {
                    [`cluster-${clusterId}-workload-proxy`]: makeGroup(
                      ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED,
                      [
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
                      ],
                      KubernetesManifestGroupSpecMode.FULL,
                    ),
                    'cert-manager': makeGroup(
                      ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING,
                      [
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
                        ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
                      ],
                      KubernetesManifestGroupSpecMode.FULL,
                    ),
                  },
                  'failed to apply manifest cert-manager/CustomResourceDefinition/certificates.cert-manager.io: the server could not find the requested resource',
                ),
              },
            ]
          },
        }).handler,
      ],
    },
  },
}

export const EmptyGroups: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterKubernetesManifestsStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterKubernetesManifestsStatusType,
            id: clusterId,
          },
          initialResources: [
            {
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterKubernetesManifestsStatusType,
                id: clusterId,
              },
              spec: {
                groups: {},
                total: 0,
              },
            },
          ],
        }).handler,
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
