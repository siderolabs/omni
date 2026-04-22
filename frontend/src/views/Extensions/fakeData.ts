// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'

import type { TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'

export const fakeExtensions = faker.helpers.multiple(
  () => ({
    name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
    author: faker.company.name(),
    version: faker.system.semver(),
    description: faker.lorem.sentences(4),
  }),
  { count: 50 },
) satisfies TalosExtensionsSpecInfo[]
