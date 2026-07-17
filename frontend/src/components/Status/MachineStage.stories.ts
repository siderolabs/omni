// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import {
  MachineStatusLabelConnected,
  MachineStatusLinkType,
  MetricsNamespace,
} from '@/api/resources'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'

import MachineStage from './MachineStage.vue'

interface MachineOptions {
  stage?: MachineStatusEventMachineStage
  powerStage?: MachineStatusSnapshotSpecPowerStage
  connected?: boolean
}

const makeMachine = ({
  stage,
  powerStage,
  connected = true,
}: MachineOptions): Resource<MachineStatusLinkSpec> => ({
  spec: {
    snapshot: {
      power_stage: powerStage,
      machine_status: stage === undefined ? undefined : { stage },
    },
  },
  metadata: {
    namespace: MetricsNamespace,
    type: MachineStatusLinkType,
    id: 'e5f6a7b8-0000-0000-0000-000000000000',
    labels: connected ? { [MachineStatusLabelConnected]: '' } : {},
  },
})

const meta: Meta<typeof MachineStage> = {
  component: MachineStage,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Running: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.RUNNING }),
  },
}

export const Booting: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.BOOTING }),
  },
}

export const Installing: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.INSTALLING }),
  },
}

export const Upgrading: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.UPGRADING }),
  },
}

export const Rebooting: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.REBOOTING }),
  },
}

export const Maintenance: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.MAINTENANCE }),
  },
}

export const ShuttingDown: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.SHUTTING_DOWN }),
  },
}

export const Resetting: Story = {
  args: {
    machine: makeMachine({ stage: MachineStatusEventMachineStage.RESETTING }),
  },
}

export const PoweredOff: Story = {
  args: {
    machine: makeMachine({
      powerStage: MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERED_OFF,
      connected: false,
    }),
  },
}

export const PoweringOn: Story = {
  args: {
    machine: makeMachine({
      powerStage: MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERING_ON,
      connected: false,
    }),
  },
}

export const Disconnected: Story = {
  args: {
    machine: makeMachine({
      stage: MachineStatusEventMachineStage.RUNNING,
      connected: false,
    }),
  },
}
