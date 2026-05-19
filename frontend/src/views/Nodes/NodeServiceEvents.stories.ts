// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import NodeServiceEvents from './NodeServiceEvents.vue'

const meta: Meta<typeof NodeServiceEvents> = {
  component: NodeServiceEvents,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: {
    events: [
      {
        state: 'Corrupted',
        msg: 'Unexpected state from runtime',
        ts: faker.date.recent().toISOString(),
      },
      { state: 'Starting', msg: 'Service is starting up', ts: faker.date.recent().toISOString() },
      {
        state: 'Preparing',
        msg: 'Waiting for dependencies',
        ts: faker.date.recent().toISOString(),
      },
      {
        state: 'Waiting',
        msg: 'Blocked on socket activation',
        ts: faker.date.recent().toISOString(),
      },
      { state: 'Running', msg: 'Service is healthy', ts: faker.date.recent().toISOString() },
      { state: 'Stopping', msg: 'Received SIGTERM', ts: faker.date.recent().toISOString() },
      { state: 'Finished', msg: 'Exited with code 0', ts: faker.date.recent().toISOString() },
      { state: 'Failed', msg: 'Exited with code 1', ts: faker.date.recent().toISOString() },
    ],
  },
}
