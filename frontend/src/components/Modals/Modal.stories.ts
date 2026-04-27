// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import Modal from './Modal.vue'

const meta: Meta<typeof Modal> = {
  component: Modal,
  args: {
    title: 'Title',
    actionLabel: 'Action',
    open: true,
    'onUpdate:open': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { Modal },
    setup: () => ({ args }),
    template: `
      <Modal v-bind="args">
        <template #description>Description</template>

        ✨Content goes here✨
      </Modal>
    `,
  }),
}
