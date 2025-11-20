<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { getDocsLink } from '@/methods'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const secureBootDocsLink = computed(() =>
  getDocsLink('talos', '/platform-specific-installations/bare-metal-platforms', {
    talosVersion: formState.value.talosVersion,
  }),
)
</script>

<template>
  <div class="flex flex-col gap-4">
    <RadioGroup v-model="formState.machineArch" label="Machine Architecture">
      <RadioGroupOption value="amd64">
        amd64

        <template #description>
          Compatible with Intel and AMD CPUs, also referred to as x86_64. If unsure, select this
          option.
        </template>
      </RadioGroupOption>

      <RadioGroupOption value="arm64">
        arm64

        <template #description>
          Suitable for Ampere Computing and other arm64 CPUs. For Single Board Computers, choose the
          'SBC' option on the first screen. For AWS and GCP arm64 VMs, use Cloud images.
        </template>
      </RadioGroupOption>
    </RadioGroup>

    <TCheckbox v-if="formState.hardwareType !== 'cloud'" v-model="formState.secureBoot">
      <div class="flex flex-col">
        <span class="font-medium text-naturals-n14">SecureBoot</span>
        <span>
          Create a
          <a
            :href="secureBootDocsLink"
            rel="noopener noreferrer"
            target="_blank"
            class="link-primary"
            @click.stop
          >
            SecureBoot
          </a>
          image signed with the official Sidero Labs signing key. This requires UEFI boot and
          pre-configured hardware.
        </span>
      </div>
    </TCheckbox>
  </div>
</template>
