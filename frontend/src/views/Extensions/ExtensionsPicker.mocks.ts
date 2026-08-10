// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'

import type { TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import type { TalosExtensionsSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, DefaultTalosVersion, TalosExtensionsType } from '@/api/resources'

export const fakeExtensions = faker.helpers.multiple(
  () => ({
    name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
    author: faker.company.name(),
    version: faker.system.semver(),
    description: faker.lorem.sentences(4),
  }),
  { count: 50 },
) satisfies TalosExtensionsSpecInfo[]

export const handlers = [
  createWatchStreamHandler<TalosExtensionsSpec>({
    expectedOptions: {
      id: DefaultTalosVersion,
      type: TalosExtensionsType,
      namespace: DefaultNamespace,
    },
    initialResources: [
      {
        spec: { items: fakeExtensions },
        metadata: {},
      },
    ],
  }).handler,
]
