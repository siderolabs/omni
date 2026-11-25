// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { DryRunStatusSpecNotCheckedReason, DryRunStatusSpecOutcome } from '@/api/omni/specs/omni.pb'

import ClusterMachineDryRunDescription from './ClusterMachineDryRunDescription.vue'
import ClusterMachineDryRunResult from './ClusterMachineDryRunResult.vue'

const meta: Meta<typeof ClusterMachineDryRunDescription> = {
  component: ClusterMachineDryRunDescription,
  parameters: {
    layout: 'centered',
  },
  render: (args) => ({
    components: { ClusterMachineDryRunDescription, ClusterMachineDryRunResult },
    setup: () => ({ args }),
    template: `
      <div class="flex items-start gap-compact text-xs">
        <ClusterMachineDryRunResult
          :dry-run="args.dryRun"
          :to="{ name: 'NodePendingUpdates', params: { cluster: 'talos-default', machine: 'machine-1' } }"
        />
        <ClusterMachineDryRunDescription v-bind="args" />
      </div>
    `,
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Failed: Story = {
  args: {
    dryRun: {
      outcome: DryRunStatusSpecOutcome.FAILED,
      error: [
        '3 errors occurred:',
        '\t* SwapVolumeConfig/swap-a: disk selector is required',
        '\t* UserVolumeConfig/data-a: disk selector is required',
        'min size is greater than max size',
        '\t* UserVolumeConfig/data-b: disk selector is required',
        'min size is greater than max size',
      ].join('\n'),
    },
  },
}

export const NotChecked: Story = {
  args: {
    dryRun: {
      outcome: DryRunStatusSpecOutcome.NOT_CHECKED,
      not_checked_reason: DryRunStatusSpecNotCheckedReason.DISCONNECTED,
    },
  },
}

export const CheckError: Story = {
  args: {
    dryRun: {
      outcome: DryRunStatusSpecOutcome.NOT_CHECKED,
      not_checked_reason: DryRunStatusSpecNotCheckedReason.CHECK_ERROR,
      error: 'failed to get client: context deadline exceeded',
    },
  },
}
