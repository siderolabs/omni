// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import path from 'node:path'

import { faker } from '@faker-js/faker'
import { milliseconds } from 'date-fns'
import fs from 'fs/promises'
import { dump } from 'js-yaml'

import { DEFAULT_MACHINE_CLASS } from '../constants'
import { expect, test as base } from '../omnictl_fixtures'

interface Cluster {
  name: string
}

interface ClusterFixtures {
  cluster: Cluster
}

const test = base.extend<ClusterFixtures>({
  cluster: [
    async ({ omnictl }, use, testInfo) => {
      const clusterName = `e2e-cluster-${faker.string.alphanumeric(8)}`

      const clusterTemplate = [
        {
          kind: 'Cluster',
          name: clusterName,
          kubernetes: { version: 'v1.35.0' },
          talos: { version: 'v1.12.3' },
        },
        {
          kind: 'ControlPlane',
          machineClass: { name: DEFAULT_MACHINE_CLASS, size: 1 },
        },
        {
          kind: 'Workers',
          machineClass: { name: DEFAULT_MACHINE_CLASS, size: 2 },
        },
      ]
        .map((doc) => dump(doc))
        .join('---\n')

      const templatePath = testInfo.outputPath('cluster.yaml')
      await fs.writeFile(templatePath, clusterTemplate)

      // Create
      await omnictl(['cluster', 'template', 'sync', '-f', templatePath, '--verbose'])

      // Wait to be ready
      await omnictl(['cluster', 'template', 'status', '-f', templatePath])

      await use({ name: clusterName })

      // Save support bundle if the test failed
      if (testInfo.status !== 'passed') {
        const bundleDir = path.resolve(testInfo.outputDir, '..', 'support-bundles')
        const bundlePath = path.join(bundleDir, `support-bundle-${clusterName}.zip`)

        try {
          await fs.mkdir(bundleDir, { recursive: true })
          await omnictl(['support', '--cluster', clusterName, '--output', bundlePath])
          await testInfo.attach(`support-bundle-${clusterName}.zip`, { path: bundlePath })
        } catch (e) {
          console.error(`failed to save support bundle for cluster ${clusterName}:`, e)
        }
      }

      // Destroy
      await omnictl(['cluster', 'template', 'delete', '-f', templatePath, '--verbose'])
    },
    { timeout: milliseconds({ minutes: 1 }) },
  ],
})

export { expect, test }
