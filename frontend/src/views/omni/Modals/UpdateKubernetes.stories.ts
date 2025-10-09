// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import * as semver from 'semver'

import type { Resource } from '@/api/grpc'
import type {
  ClusterSpec,
  KubernetesUpgradeStatusSpec,
  KubernetesVersionSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterType,
  DefaultNamespace,
  KubernetesUpgradeStatusType,
  KubernetesVersionType,
  TalosVersionType,
} from '@/api/resources'

import UpdateKubernetes from './UpdateKubernetes.vue'

const k8sVersions = faker.helpers
  .uniqueArray(
    () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .sort(semver.compare)

const minTalos = 10
const maxTalos = 11
const talosVersions = faker.helpers
  .uniqueArray(
    () =>
      `1.${faker.number.int({ min: minTalos, max: maxTalos })}.${faker.number.int({ min: 0, max: 10 })}`,
    10,
  )
  .sort(semver.compare)

const meta: Meta<typeof UpdateKubernetes> = {
  component: UpdateKubernetes,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterSpec>({
          expectedOptions: {
            type: ClusterType,
            namespace: DefaultNamespace,
          },
          initialResources: [
            {
              spec: { talos_version: talosVersions.find((v) => semver.minor(v) === minTalos) },
              metadata: {},
            },
          ],
        }).handler,

        createWatchStreamHandler<KubernetesUpgradeStatusSpec>({
          expectedOptions: {
            type: KubernetesUpgradeStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: () => {
            const upgrade_versions = k8sVersions.filter((v) => [28, 29].includes(semver.minor(v)))

            const [last_upgrade_version] = upgrade_versions
              .filter((v) => semver.minor(v) === 28)
              .splice(-3, 1)

            return [
              {
                spec: { last_upgrade_version, upgrade_versions },
                metadata: {},
              },
            ]
          },
        }).handler,

        createWatchStreamHandler<KubernetesVersionSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: KubernetesVersionType,
          },
          initialResources: k8sVersions.map<Resource<KubernetesVersionSpec>>((version) => ({
            spec: { version },
            metadata: { id: version },
          })),
        }).handler,

        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: talosVersions.map<Resource<TalosVersionSpec>>((version) => {
            const minor = semver.minor(version)

            return {
              spec: {
                version,
                compatible_kubernetes_versions:
                  minor === minTalos
                    ? k8sVersions.slice(0, Math.floor(k8sVersions.length / 2))
                    : k8sVersions.slice(Math.ceil(k8sVersions.length / 2)),
              },
              metadata: { id: version },
            }
          }),
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
