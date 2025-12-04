<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { gte } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type PlatformConfigSpec, PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, MetalPlatformConfigType, VirtualNamespace } from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { getDocsLink } from '@/methods'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceList } from '@/methods/useResourceList'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const secureBootDocsLink = computed(() =>
  getDocsLink('talos', '/platform-specific-installations/bare-metal-platforms', {
    talosVersion: formState.value.talosVersion,
  }),
)

const { data: metalProvider } = useResourceGet<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'metal',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: MetalPlatformConfigType,
    id: 'metal',
  },
}))

const { data: cloudProviders } = useResourceList<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'cloud',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: CloudPlatformConfigType,
  },
}))

const selectedCloudProvider = computed(() =>
  cloudProviders.value?.find((provider) => formState.value.cloudPlatform === provider.metadata.id),
)

const isGte1_5 = computed(
  () => formState.value.talosVersion && gte(formState.value.talosVersion, '1.5.0'),
)

const secureBootSupported = computed(
  () =>
    isGte1_5.value &&
    (formState.value.hardwareType === 'metal' ||
      selectedCloudProvider.value?.spec.secure_boot_supported),
)

const supportedArchitectures = computed(() => {
  switch (formState.value.hardwareType) {
    case 'sbc':
      return [PlatformConfigSpecArch.ARM64]
    case 'metal':
      return metalProvider.value?.spec.architectures ?? []
    case 'cloud':
      return selectedCloudProvider.value?.spec.architectures ?? []
    default:
      return []
  }
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <RadioGroup v-model="formState.machineArch" label="Machine Architecture">
      <RadioGroupOption
        v-if="supportedArchitectures.includes(PlatformConfigSpecArch.AMD64)"
        :value="PlatformConfigSpecArch.AMD64"
      >
        amd64

        <template #description>
          Compatible with Intel and AMD CPUs, also referred to as x86_64. If unsure, select this
          option.
        </template>
      </RadioGroupOption>

      <RadioGroupOption
        v-if="supportedArchitectures.includes(PlatformConfigSpecArch.ARM64)"
        :value="PlatformConfigSpecArch.ARM64"
      >
        arm64

        <template #description>
          Suitable for Ampere Computing and other arm64 CPUs. For Single Board Computers, choose the
          'SBC' option on the first screen. For AWS and GCP arm64 VMs, use Cloud images.
        </template>
      </RadioGroupOption>
    </RadioGroup>

    <TCheckbox v-if="secureBootSupported" v-model="formState.secureBoot">
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
