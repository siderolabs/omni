// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { formatRFC3339 } from 'date-fns'
import type {
  Container as V1Container,
  ContainerStatus as V1ContainerStatus,
  PodSpec as V1PodSpec,
  PodStatus as V1PodStatus,
} from 'kubernetes-types/core/v1'

import { kubernetes } from '@/api/resources'
import { TPodsViewFilterOptions } from '@/constants'

import TPods from './TPods.vue'

const meta: Meta<typeof TPods> = {
  component: TPods,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<V1PodSpec, V1PodStatus>({
          expectedOptions: { type: kubernetes.pod },
          initialResources: () => {
            faker.seed(0)

            const containers: V1Container[] = [
              {
                name: faker.helpers.slugify(
                  [faker.hacker.verb(), faker.hacker.adjective()].join(' '),
                ),
                image: `${faker.internet.domainName()}/${faker.internet.domainWord()}:v${faker.system.semver()}`,
              },
            ]

            return faker.helpers.multiple(
              () => ({
                metadata: {
                  name: faker.helpers.slugify(
                    [faker.hacker.verb(), faker.hacker.adjective(), faker.hacker.noun()].join(' '),
                  ),
                  namespace: 'kube-system',
                },
                spec: {
                  nodeName: `machine-${faker.string.uuid()}`,
                  containers,
                },
                status: {
                  startTime: formatRFC3339(faker.date.past()),
                  podIP: faker.internet.ipv4(),
                  containerStatuses: containers.map(
                    () =>
                      ({
                        ready: faker.datatype.boolean(),
                        restartCount: faker.number.int(10),
                      }) as V1ContainerStatus,
                  ),
                  phase: faker.helpers.arrayElement(
                    Object.values(TPodsViewFilterOptions).filter(
                      (o) => o !== TPodsViewFilterOptions.ALL,
                    ),
                  ),
                },
              }),
              { count: 20 },
            )
          },
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
