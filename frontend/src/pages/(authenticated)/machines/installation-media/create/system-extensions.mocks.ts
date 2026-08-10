// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'

import type { TalosExtensionsSpec, TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosExtensionsType } from '@/api/resources'

export const handlers = [
  createWatchStreamHandler<TalosExtensionsSpec>({
    expectedOptions: {
      type: TalosExtensionsType,
      namespace: DefaultNamespace,
    },
    initialResources: [
      {
        spec: {
          items: faker.helpers.multiple<TalosExtensionsSpecInfo>(
            () => ({
              name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
              author: faker.company.name(),
              version: faker.system.semver(),
              description: faker.lorem.sentences(4),
            }),
            { count: 50 },
          ),
        },
        metadata: {},
      },
    ],
  }).handler,
]
