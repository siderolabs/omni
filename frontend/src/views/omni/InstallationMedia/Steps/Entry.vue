<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })
</script>

<template>
  <div class="flex flex-col gap-4">
    <TInput v-model="formState.name" title="Name" overhead-title />

    <RadioGroup
      v-model="formState.hardwareType"
      label="Hardware Type"
      :options="
        [
          {
            label: 'Bare-metal Machine',
            description:
              'Suitable for x86-64 and arm64 bare-metal machines, as well as generic virtual machines. If unsure, choose this option.',
            value: 'metal',
          },
          {
            label: 'Cloud Server',
            description:
              'Compatible with AWS, GCP, Azure, VMWare, Equinix Metal, and other platforms, including homelab environments like Proxmox.',
            value: 'cloud',
          },
          {
            label: 'Single Board Computer',
            description: 'Supports Raspberry Pi, Pine64, Jetson Nano, and similar devices.',
            value: 'sbc',
          },
        ] satisfies { [key: string]: any; value: typeof formState.hardwareType }[]
      "
    />
  </div>
</template>
