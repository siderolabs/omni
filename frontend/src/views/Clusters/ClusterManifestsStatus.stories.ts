// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { dump } from 'js-yaml'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type {
  ClusterKubernetesManifestsStatusSpec,
  ClusterKubernetesManifestsStatusSpecGroupStatus,
  ClusterKubernetesManifestsStatusSpecManifestStatus,
  KubernetesManifestGroupSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
  KubernetesManifestGroupSpecMode,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterKubernetesManifestsStatusType,
  DefaultNamespace,
  KubernetesManifestGroupType,
} from '@/api/resources'

import ClusterManifestsStatus from './ClusterManifestsStatus.vue'

const clusterId = 'talos-default'

const meta: Meta<typeof ClusterManifestsStatus> = {
  component: ClusterManifestsStatus,
  args: {
    cluster: clusterId,
  },
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    () => ({
      template: '<div class="h-screen"><story /></div>',
    }),
  ],
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

// Builds a plausible manifest body for the given kind so the YAML preview panel has
// something realistic to display, and matches the ManifestStatus it's paired with.
function makeManifestYAML(
  resource: (typeof k8sKinds)[number],
  name: string,
  namespace?: string,
): string {
  const apiVersion = resource.group ? `${resource.group}/v1` : 'v1'

  const metadata: Record<string, unknown> = {
    name,
    ...(namespace ? { namespace } : {}),
    labels: { 'app.kubernetes.io/managed-by': 'omni' },
  }

  let spec: Record<string, unknown> | undefined

  switch (resource.kind) {
    case 'Deployment':
    case 'DaemonSet':
    case 'StatefulSet':
      spec = {
        replicas: faker.number.int({ min: 1, max: 3 }),
        selector: { matchLabels: { app: name } },
        template: {
          metadata: { labels: { app: name } },
          spec: {
            containers: [
              {
                name,
                image: `${faker.helpers.slugify(faker.commerce.productName()).toLowerCase()}:${faker.system.semver()}`,
              },
            ],
          },
        },
      }
      break
    case 'Service':
      spec = {
        selector: { app: name },
        ports: [{ port: faker.internet.port(), targetPort: faker.internet.port() }],
      }
      break
    case 'ConfigMap':
      return dump({
        apiVersion,
        kind: resource.kind,
        metadata,
        data: { 'config.yaml': 'key: value' },
      })
    case 'Secret':
      return dump({
        apiVersion,
        kind: resource.kind,
        metadata,
        type: 'Opaque',
        data: { token: btoa(faker.string.alphanumeric(16)) },
      })
    default:
      spec = undefined
  }

  return dump({
    apiVersion,
    kind: resource.kind,
    metadata,
    ...(spec ? { spec } : {}),
  })
}

interface GroupResult {
  status: ClusterKubernetesManifestsStatusSpecGroupStatus
  yaml: string
}

const makeGroup = (
  phase: ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  manifestPhases: ClusterKubernetesManifestsStatusSpecManifestStatusPhase[],
  mode: KubernetesManifestGroupSpecMode,
): GroupResult => {
  const manifests: Record<string, ClusterKubernetesManifestsStatusSpecManifestStatus> = {}
  const yamlDocs: string[] = []

  for (const manifestPhase of manifestPhases) {
    const resource = faker.helpers.arrayElement(k8sKinds)
    const name = faker.helpers
      .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
      .toLowerCase()
    const ns = resource.namespaced ? faker.helpers.arrayElement(k8sNamespaces) : undefined
    const key = ns ? `${resource.kind}/${ns}/${name}` : `${resource.kind}/${name}`

    manifests[key] = {
      phase: manifestPhase,
      kind: resource.kind,
      name,
      namespace: ns,
      group: resource.group || undefined,
    }

    yamlDocs.push(makeManifestYAML(resource, name, ns))
  }

  return {
    status: { phase, mode, manifests },
    yaml: yamlDocs.join('---\n'),
  }
}

const makeSpec = (
  seed: number,
  groups: Record<string, ClusterKubernetesManifestsStatusSpecGroupStatus>,
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

// Answers the KubernetesManifestGroup Get request the details panel issues when a
// manifest node is clicked, returning the YAML generated alongside its group status.
function manifestGroupGetHandler(yamlByGroupId: Record<string, string>) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { type, namespace, id } = await request.clone().json()

      if (type !== KubernetesManifestGroupType || namespace !== DefaultNamespace || !id) return

      const data = yamlByGroupId[id]

      if (data === undefined) return

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: { namespace: DefaultNamespace, type: KubernetesManifestGroupType, id },
          spec: {
            data,
            mode: KubernetesManifestGroupSpecMode.FULL,
          } satisfies KubernetesManifestGroupSpec,
        } satisfies Resource<KubernetesManifestGroupSpec>),
      })
    },
  )
}

const defaultGroupResults = (() => {
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

  return faker.helpers.multiple(
    () => {
      const name = faker.helpers
        .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
        .toLowerCase()
      const groupPhase = faker.helpers.arrayElement(groupPhases)
      const manifestCount = faker.number.int({ min: 2, max: 12 })
      const manifests = faker.helpers.multiple(() => faker.helpers.arrayElement(allPhases), {
        count: manifestCount,
      })

      return [
        name,
        makeGroup(groupPhase, manifests, faker.helpers.enumValue(KubernetesManifestGroupSpecMode)),
      ] as const
    },
    { count: 12 },
  )
})()

const defaultGroups = Object.fromEntries(
  defaultGroupResults.map(([name, result]) => [name, result.status]),
)

const defaultYAMLByGroupId = Object.fromEntries(
  defaultGroupResults.map(([name, result]) => [name, result.yaml]),
)

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
          initialResources: [
            {
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterKubernetesManifestsStatusType,
                id: clusterId,
              },
              spec: makeSpec(4, defaultGroups),
            },
          ],
        }).handler,
        manifestGroupGetHandler(defaultYAMLByGroupId),
      ],
    },
  },
}

const withErrorGroupIds = {
  workloadProxy: `cluster-${clusterId}-workload-proxy`,
  certManager: 'cert-manager',
}

const withErrorGroups = (() => {
  faker.seed(3)

  const workloadProxy = makeGroup(
    ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED,
    [
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
    ],
    KubernetesManifestGroupSpecMode.FULL,
  )
  const certManager = makeGroup(
    ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING,
    [
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
      ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
    ],
    KubernetesManifestGroupSpecMode.FULL,
  )

  return { workloadProxy, certManager }
})()

const withErrorYAMLByGroupId = {
  [withErrorGroupIds.workloadProxy]: withErrorGroups.workloadProxy.yaml,
  [withErrorGroupIds.certManager]: withErrorGroups.certManager.yaml,
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
          initialResources: [
            {
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterKubernetesManifestsStatusType,
                id: clusterId,
              },
              spec: makeSpec(
                3,
                {
                  [withErrorGroupIds.workloadProxy]: withErrorGroups.workloadProxy.status,
                  [withErrorGroupIds.certManager]: withErrorGroups.certManager.status,
                },
                'failed to apply manifest cert-manager/CustomResourceDefinition/certificates.cert-manager.io: the server could not find the requested resource',
              ),
            },
          ],
        }).handler,
        manifestGroupGetHandler(withErrorYAMLByGroupId),
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
