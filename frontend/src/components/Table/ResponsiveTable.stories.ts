// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TButton from '@/components/Button/TButton.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import TableCell from '@/components/Table/TableCell.vue'

import ResponsiveTable from './ResponsiveTable.vue'

interface Service {
  name: string
  healthy: boolean
  replicas: number
  uptime: string
  message?: string
}

const rows: Service[] = [
  { name: 'api', healthy: true, replicas: 3, uptime: '12d 6h' },
  {
    name: 'worker',
    healthy: false,
    replicas: 1,
    uptime: '41s',
    message: 'queue connection refused: dial tcp 10.0.0.7:5672: connect: connection refused',
  },
  { name: 'scheduler', healthy: true, replicas: 1, uptime: '3h 5m' },
]

const meta: Meta<typeof ResponsiveTable> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: ResponsiveTable as any,
  args: { rows, rowKey: (row: Service) => row.name, minWidthRem: 45 },
  argTypes: {
    minWidthRem: { control: { type: 'number', step: 2.5 } },
  },
  parameters: {
    docs: {
      description: {
        component:
          'Narrower than `minWidthRem`, the table gives way to `#stacked`, one card per row. Resize the canvas or change `minWidthRem` to see it switch.',
      },
    },
  },
  render: (args) => ({
    components: {
      // https://github.com/storybookjs/storybook/issues/24238
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ResponsiveTable: ResponsiveTable as any,
      StatusPill,
      TButton,
      TableCell,
    },
    setup: () => ({ args }),
    template: `
      <ResponsiveTable v-bind="args">
        <template #head>
          <TableCell th class="w-40">Name</TableCell>
          <TableCell th class="w-28">Status</TableCell>
          <TableCell th class="w-24">Replicas</TableCell>
          <TableCell th class="w-24">Uptime</TableCell>
          <TableCell th>Message</TableCell>
          <TableCell th class="w-24"><span class="sr-only">Actions</span></TableCell>
        </template>

        <template #row="{ row }">
          <TableCell class="text-content-emphasis">{{ row.name }}</TableCell>
          <TableCell>
            <StatusPill :tone="row.healthy ? 'success' : 'danger'">
              {{ row.healthy ? 'Healthy' : 'Failing' }}
            </StatusPill>
          </TableCell>
          <TableCell>{{ row.replicas }}</TableCell>
          <TableCell>{{ row.uptime }}</TableCell>
          <TableCell><span class="block truncate">{{ row.message }}</span></TableCell>
          <TableCell><TButton variant="secondary" size="sm">Open</TButton></TableCell>
        </template>

        <template #stacked="{ row }">
          <div class="flex flex-col gap-tight">
            <div class="flex items-start justify-between gap-tight">
              <span class="text-content-emphasis">{{ row.name }}</span>
              <StatusPill :tone="row.healthy ? 'success' : 'danger'">
                {{ row.healthy ? 'Healthy' : 'Failing' }}
              </StatusPill>
            </div>
            <p v-if="row.message" class="wrap-anywhere">{{ row.message }}</p>
            <div class="flex items-center justify-between gap-tight">
              <p class="text-content-secondary">
                {{ row.replicas }} {{ row.replicas === 1 ? 'replica' : 'replicas' }}, {{ row.uptime }} up
              </p>
              <TButton variant="secondary" size="sm">Open</TButton>
            </div>
          </div>
        </template>
      </ResponsiveTable>
    `,
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Stacked: Story = {
  globals: { viewport: { value: 'mobile2', isRotated: false } },
}
