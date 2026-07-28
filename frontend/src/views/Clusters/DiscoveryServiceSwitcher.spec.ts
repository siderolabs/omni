// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { expect, test } from 'vitest'

import DiscoveryServiceSwitcher from './DiscoveryServiceSwitcher.vue'

const publicConfigurableProps = {
  embeddedAvailable: true,
  publicConfigurable: true,
}

// Switching between Public and Embedded flips both booleans at once. Reporting them as two separate changes
// would leave one of the two carrying "no embedded, no public", which the backend rejects, so each click has
// to produce exactly one event.
test('reports a switch from public to embedded as a single change', async () => {
  const user = userEvent.setup()

  const { emitted } = render(DiscoveryServiceSwitcher, {
    props: { useEmbedded: false, disablePublic: false, ...publicConfigurableProps },
  })

  await user.click(screen.getByRole('radio', { name: 'Embedded' }))

  expect(emitted('change')).toEqual([[{ useEmbedded: true, disablePublic: true }]])
})

test('reports a switch from embedded to public as a single change', async () => {
  const user = userEvent.setup()

  const { emitted } = render(DiscoveryServiceSwitcher, {
    props: { useEmbedded: true, disablePublic: true, ...publicConfigurableProps },
  })

  await user.click(screen.getByRole('radio', { name: 'Public' }))

  expect(emitted('change')).toEqual([[{ useEmbedded: false, disablePublic: false }]])
})

test('keeps the public service when both are selected', async () => {
  const user = userEvent.setup()

  const { emitted } = render(DiscoveryServiceSwitcher, {
    props: { useEmbedded: false, disablePublic: false, ...publicConfigurableProps },
  })

  await user.click(screen.getByRole('radio', { name: 'Both' }))

  expect(emitted('change')).toEqual([[{ useEmbedded: true, disablePublic: false }]])
})

// On pre-1.14 clusters the embedded service implicitly replaces the public one, so the opt-out stays off and
// the Both option is not offered at all.
test('leaves the public opt-out off when it is not configurable', async () => {
  const user = userEvent.setup()

  const { emitted } = render(DiscoveryServiceSwitcher, {
    props: { useEmbedded: false, disablePublic: false, embeddedAvailable: true },
  })

  expect(screen.queryByRole('radio', { name: 'Both' })).toBeNull()

  await user.click(screen.getByRole('radio', { name: 'Embedded' }))

  expect(emitted('change')).toEqual([[{ useEmbedded: true, disablePublic: false }]])
})
