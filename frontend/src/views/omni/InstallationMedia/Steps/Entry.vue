<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount } from 'vue'

import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import type { FormState } from '@/views/omni/InstallationMedia/useFormState'

const formState = defineModel<FormState>({ required: true })

// Form defaults
onBeforeMount(() => (formState.value.hardwareType ??= 'metal'))
</script>

<template>
  <div class="flex flex-col gap-4">
    <RadioGroup v-model="formState.hardwareType" label="Hardware Type">
      <RadioGroupOption value="metal">
        Bare-metal Machine

        <template #description>
          Suitable for x86-64 and arm64 bare-metal machines, as well as generic virtual machines. If
          unsure, choose this option.
        </template>
      </RadioGroupOption>

      <RadioGroupOption value="cloud">
        Cloud Server

        <template #description>
          Compatible with AWS, GCP, Azure, VMWare, Equinix Metal, and other platforms, including
          homelab environments like Proxmox.
        </template>
      </RadioGroupOption>

      <RadioGroupOption value="sbc">
        Single Board Computer

        <template #description>
          Supports Raspberry Pi, Pine64, Jetson Nano, and similar devices.
        </template>
      </RadioGroupOption>
    </RadioGroup>
  </div>
</template>
