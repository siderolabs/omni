<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import TInput from '@/components/common/TInput/TInput.vue'

const name = ref('')
const hardwareType = ref<string>()
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="p-6">
      <h1 class="mb-6 text-xl font-medium text-naturals-n14">Create New Media</h1>

      <div class="flex flex-col gap-4">
        <TInput v-model="name" title="Name" overhead-title />

        <RadioGroup
          v-model="hardwareType"
          label="Hardware Type"
          :options="[
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
          ]"
        />
      </div>
    </div>

    <div class="shrink grow"></div>

    <div class="flex h-16 items-center justify-end border-t border-naturals-n4 bg-naturals-n1 px-4">
      <TButton type="highlighted">Next</TButton>
    </div>
  </div>
</template>
