// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { ListRequest, ListResponse } from '@/api/omni/resources/resources.pb'
import type {
  ClusterMachineConfigStatusSpec,
  ClusterStatusSpec,
  FeaturesConfigSpec,
  MachineStatusSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineConfigStatusType,
  ClusterStatusType,
  DefaultNamespace,
  FeaturesConfigID,
  FeaturesConfigType,
  LabelCluster,
  LabelControlPlaneRole,
  LabelWorkerRole,
  MachineStatusType,
  TalosVersionType,
} from '@/api/resources'
import ClusterSecurity from '@/views/ClusterSecurity/ClusterSecurity.vue'
import type { Match } from '@/views/InstallationMedia/vulnerabilities/ReportTypes'

import sampleReport from '../InstallationMedia/vulnerabilities/sample-report.json'

const CLUSTER = 'demo-cluster'
const CURRENT_VERSION = '1.9.0'
const PATCH_VERSION = '1.9.3'
const MINOR_VERSION = '1.10.2'

const SCHEMATIC_CP = 'a'.repeat(64)
const SCHEMATIC_WORKER = 'b'.repeat(64)

const allMatches = sampleReport.matches as Match[]

// A fabricated finding used to demonstrate a vulnerability *introduced* by an upgrade.
const introducedMatch: Match = {
  vulnerability: {
    id: 'CVE-2025-99999',
    dataSource: 'https://nvd.nist.gov/vuln/detail/CVE-2025-99999',
    namespace: 'nvd:cpe',
    severity: 'High',
    urls: [],
    description: 'A vulnerability newly introduced in this Talos release (demo data).',
    cvss: [
      {
        type: 'Primary',
        version: '3.1',
        vector: '',
        metrics: { baseScore: 7.5 },
        vendorMetadata: {},
      },
    ],
    fix: { versions: [], state: 'unknown' },
    advisories: [],
    risk: 0,
  },
  relatedVulnerabilities: [],
  matchDetails: [],
  artifact: {
    id: 'demo',
    name: 'github.com/example/newly-introduced',
    version: '2.1.0',
    type: 'go-module',
    locations: null,
    language: 'go',
    licenses: [],
    cpes: [],
    purl: '',
    upstreams: [],
  },
}

// Each upgrade target removes some findings (fixed) — the minor bump additionally
// introduces one new finding — so the diff has something to show.
function matchesForVersion(version: string): Match[] {
  switch (version) {
    case PATCH_VERSION:
      return allMatches.slice(8)
    case MINOR_VERSION:
      return [...allMatches.slice(25), introducedMatch]
    default:
      return allMatches
  }
}

function machineStatus(id: string, arch: string): Resource<MachineStatusSpec> {
  return {
    metadata: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id,
      labels: { [LabelCluster]: CLUSTER },
    },
    spec: { hardware: { arch } },
  }
}

function configStatus(
  id: string,
  schematicId: string,
  role: string,
): Resource<ClusterMachineConfigStatusSpec> {
  return {
    metadata: {
      namespace: DefaultNamespace,
      type: ClusterMachineConfigStatusType,
      id,
      labels: { [LabelCluster]: CLUSTER, [role]: '' },
    },
    spec: { schematic_id: schematicId, talos_version: CURRENT_VERSION },
  }
}

const featuresHandler = createWatchStreamHandler<FeaturesConfigSpec>({
  expectedOptions: { namespace: DefaultNamespace, type: FeaturesConfigType, id: FeaturesConfigID },
  initialResources: [
    {
      metadata: { namespace: DefaultNamespace, type: FeaturesConfigType, id: FeaturesConfigID },
      spec: {
        is_enterprise_image_factory: true,
        image_factory_base_url: 'https://factory-enterprise.talos.dev',
      },
    },
  ],
}).handler

const clusterStatusHandler = createWatchStreamHandler<ClusterStatusSpec>({
  expectedOptions: { namespace: DefaultNamespace, type: ClusterStatusType, id: CLUSTER },
  initialResources: [
    {
      metadata: { namespace: DefaultNamespace, type: ClusterStatusType, id: CLUSTER },
      spec: { talos_version: CURRENT_VERSION, available: true },
    },
  ],
}).handler

const TALOS_VERSIONS = ['1.8.5', CURRENT_VERSION, '1.9.1', PATCH_VERSION, '1.10.0', MINOR_VERSION]

const talosVersionsHandler = http.post<never, ListRequest, ListResponse>(
  '/omni.resources.ResourceService/List',
  async ({ request }) => {
    const { type, namespace } = await request.clone().json()

    if (type !== TalosVersionType || namespace !== DefaultNamespace) return

    return HttpResponse.json({
      total: TALOS_VERSIONS.length,
      items: TALOS_VERSIONS.map((version) =>
        JSON.stringify({
          metadata: { namespace, type, id: version },
          spec: { version },
        } satisfies Resource<TalosVersionSpec>),
      ),
    })
  },
)

// Resolves a vulnerability report for any (schematic, version, arch) the page asks for.
const scanHandler = http.get('/api/vulns/:schematic/:version/:arch/report.json', ({ params }) => {
  return HttpResponse.json({
    status: 'done',
    report: { matches: matchesForVersion(params.version as string) },
  })
})

const meta: Meta<typeof ClusterSecurity> = {
  component: ClusterSecurity,
  args: {
    clusterId: CLUSTER,
  },
}

export default meta
type Story = StoryObj<typeof meta>

/** A homogeneous cluster: one schematic, one architecture, three machines. */
export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        featuresHandler,
        clusterStatusHandler,
        talosVersionsHandler,
        createWatchStreamHandler<ClusterMachineConfigStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterMachineConfigStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [
            configStatus('machine-1', SCHEMATIC_CP, LabelControlPlaneRole),
            configStatus('machine-2', SCHEMATIC_CP, LabelWorkerRole),
            configStatus('machine-3', SCHEMATIC_CP, LabelWorkerRole),
          ],
        }).handler,
        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: MachineStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [
            machineStatus('machine-1', 'amd64'),
            machineStatus('machine-2', 'amd64'),
            machineStatus('machine-3', 'amd64'),
          ],
        }).handler,
        scanHandler,
      ],
    },
  },
}

/** A heterogeneous cluster: control plane and workers use different schematics and architectures. */
export const HeterogeneousCluster: Story = {
  parameters: {
    msw: {
      handlers: [
        featuresHandler,
        clusterStatusHandler,
        talosVersionsHandler,
        createWatchStreamHandler<ClusterMachineConfigStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterMachineConfigStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [
            configStatus('machine-1', SCHEMATIC_CP, LabelControlPlaneRole),
            configStatus('machine-2', SCHEMATIC_WORKER, LabelWorkerRole),
            configStatus('machine-3', SCHEMATIC_WORKER, LabelWorkerRole),
          ],
        }).handler,
        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: MachineStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [
            machineStatus('machine-1', 'amd64'),
            machineStatus('machine-2', 'arm64'),
            machineStatus('machine-3', 'arm64'),
          ],
        }).handler,
        scanHandler,
      ],
    },
  },
}

/** A cluster already on the latest available Talos version — no upgrade paths. */
export const NoUpgradesAvailable: Story = {
  parameters: {
    msw: {
      handlers: [
        featuresHandler,
        createWatchStreamHandler<ClusterStatusSpec>({
          expectedOptions: { namespace: DefaultNamespace, type: ClusterStatusType, id: CLUSTER },
          initialResources: [
            {
              metadata: { namespace: DefaultNamespace, type: ClusterStatusType, id: CLUSTER },
              spec: { talos_version: MINOR_VERSION, available: true },
            },
          ],
        }).handler,
        talosVersionsHandler,
        createWatchStreamHandler<ClusterMachineConfigStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterMachineConfigStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [configStatus('machine-1', SCHEMATIC_CP, LabelControlPlaneRole)],
        }).handler,
        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: MachineStatusType,
            selectors: { [LabelCluster]: CLUSTER },
          },
          initialResources: [machineStatus('machine-1', 'amd64')],
        }).handler,
        scanHandler,
      ],
    },
  },
}
