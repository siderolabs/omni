// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createBytesPayload, createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import {
  Arch,
  type ArtifactTarget,
  type ClusterArtifactTargetsRequest,
  type ClusterArtifactTargetsResponse,
  type VulnerabilityReportRequest,
  type VulnerabilityReportResponse,
} from '@/api/omni/imagefactory/imagefactory.pb'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { FeaturesConfigSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  FeaturesConfigID,
  FeaturesConfigType,
  TalosVersionType,
} from '@/api/resources'
import ClusterSecurity from '@/views/ClusterSecurity/ClusterSecurity.vue'
import type { Match, VulnerabilityReport } from '@/views/ClusterSecurity/util/ReportTypes'

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

function artifactTarget(
  schematicId: string,
  arch: Arch,
  machineCount: number,
  includesControlPlane: boolean,
): ArtifactTarget {
  return {
    schematic_id: schematicId,
    arch,
    machine_count: machineCount,
    includes_control_plane: includesControlPlane,
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

// The single Talos version the page looks up to check it's served by the enterprise factory.
const talosVersionHandler = http.post<never, GetRequest, GetResponse>(
  '/omni.resources.ResourceService/Get',
  async ({ request }) => {
    const { namespace, type, id } = await request.clone().json()

    if (namespace !== DefaultNamespace || type !== TalosVersionType) return

    return HttpResponse.json({
      body: JSON.stringify({
        spec: {
          version: id,
          is_enterprise: true,
        },
        metadata: { namespace, type, id },
      } satisfies Resource<TalosVersionSpec>),
    })
  },
)

// Resolves the cluster's installed (schematic, arch) targets and upgrade paths.
function artifactTargetsHandler(
  targets: ArtifactTarget[],
  currentTalosVersion: string,
  upgradeTargetVersions: string[] = [],
) {
  return http.post<never, ClusterArtifactTargetsRequest, ClusterArtifactTargetsResponse>(
    '/imagefactory.ImageFactoryService/ClusterArtifactTargets',
    async ({ request }) => {
      const { cluster_id } = await request.clone().json()

      if (cluster_id !== CLUSTER) return

      return HttpResponse.json({
        targets,
        current_talos_version: currentTalosVersion,
        upgrade_target_versions: upgradeTargetVersions,
      })
    },
  )
}

// Resolves a vulnerability report for any (schematic, version, arch) the page asks for.
const scanHandler = http.post<never, VulnerabilityReportRequest, VulnerabilityReportResponse>(
  '/imagefactory.ImageFactoryService/VulnerabilityReport',
  async ({ request }) => {
    const { talos_version } = await request.clone().json()

    return HttpResponse.json({
      data: createBytesPayload({
        matches: matchesForVersion(talos_version!),
      } satisfies Pick<VulnerabilityReport, 'matches'>),
    })
  },
)

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
  beforeEach({ msw }) {
    msw.use(
      featuresHandler,
      talosVersionHandler,
      artifactTargetsHandler([artifactTarget(SCHEMATIC_CP, Arch.AMD64, 3, true)], CURRENT_VERSION, [
        PATCH_VERSION,
        MINOR_VERSION,
      ]),
      scanHandler,
    )
  },
}

/** A heterogeneous cluster: control plane and workers use different schematics and architectures. */
export const HeterogeneousCluster: Story = {
  beforeEach({ msw }) {
    msw.use(
      featuresHandler,
      talosVersionHandler,
      artifactTargetsHandler(
        [
          artifactTarget(SCHEMATIC_CP, Arch.AMD64, 1, true),
          artifactTarget(SCHEMATIC_WORKER, Arch.ARM64, 2, false),
        ],
        CURRENT_VERSION,
        [PATCH_VERSION, MINOR_VERSION],
      ),
      scanHandler,
    )
  },
}

/** A cluster already on the latest available Talos version — no upgrade paths. */
export const NoUpgradesAvailable: Story = {
  beforeEach({ msw }) {
    msw.use(
      featuresHandler,
      talosVersionHandler,
      artifactTargetsHandler([artifactTarget(SCHEMATIC_CP, Arch.AMD64, 1, true)], MINOR_VERSION),
      scanHandler,
    )
  },
}
