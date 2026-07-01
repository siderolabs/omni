// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { MachineSetSpecUpdateStrategy } from '@/api/omni/specs/omni.pb.ts'
import { LabelControlPlaneRole, LabelWorkerRole } from '@/api/resources.ts'
import type { MachineSet } from '@/states/cluster-management.ts'

import MachineSetConfigEditModal from './MachineSetConfigEditModal.vue'

faker.seed(0)

const meta: Meta<typeof MachineSetConfigEditModal> = {
  component: MachineSetConfigEditModal,
  args: {
    open: true,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Worker: Story = {
  args: {
    machineSet: {
      id: faker.string.uuid(),
      role: LabelWorkerRole,
      upgradeStrategy: {
        type: MachineSetSpecUpdateStrategy.Rolling,
        config: { rolling: { max_parallelism: 3 } },
      },
      updateStrategy: {
        type: MachineSetSpecUpdateStrategy.Rolling,
        config: { rolling: { max_parallelism: 3 } },
      },
      deleteStrategy: {
        type: MachineSetSpecUpdateStrategy.Rolling,
        config: { rolling: { max_parallelism: 3 } },
      },
    } as MachineSet,
  },
}

export const ControlPlane: Story = {
  args: {
    machineSet: {
      id: faker.string.uuid(),
      role: LabelControlPlaneRole,
      upgradeStrategy: {
        type: MachineSetSpecUpdateStrategy.Rolling,
        config: { rolling: { max_parallelism: 3 } },
      },
    } as MachineSet,
  },
}
