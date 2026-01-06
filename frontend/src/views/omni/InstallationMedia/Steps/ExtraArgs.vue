<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { gte } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { SchematicBootloader } from '@/api/omni/management/management.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { SBCConfigType, VirtualNamespace } from '@/api/resources'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import TextArea from '@/components/common/TextArea/TextArea.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { getDocsLink } from '@/methods'
import { useResourceList } from '@/methods/useResourceList'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const supportsCustomisingKernelArgs = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.10.0'),
)

const supportsBootloaderSelection = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.12.0-alpha.2'),
)

const { data: SBCs } = useResourceList<SBCConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'sbc',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
  },
}))

const selectedSBC = computed(() =>
  SBCs.value?.find((sbc) => sbc.metadata.id === formState.value.sbcType),
)
</script>

<template>
  <div class="flex flex-col gap-4 text-xs [&_code]:font-bold">
    <span class="text-sm font-medium text-naturals-n14">Customization</span>

    <TInput
      v-model="formState.cmdline"
      placeholder="-console console=tty0"
      title="Extra kernel command line arguments"
      overhead-title
    />

    <p>
      This step allows you to customize the default
      <a
        target="_blank"
        rel="noopener noreferrer"
        :href="getDocsLink('talos', '/reference/kernel', { talosVersion: formState.talosVersion })"
        class="link-primary"
      >
        kernel command line arguments.
      </a>
    </p>

    <!-- prettier-ignore -->
    <p>
      The syntax accepted is the same as the kernel command line syntax,
      e.g., <code>console=ttyS0,115200</code>.
      Prefixing an argument with <code>-</code> removes it from
      the default command line (e.g., <code>-console</code>).
    </p>

    <p v-if="formState.secureBoot">
      With SecureBoot, the kernel command line is signed and cannot be modified, so this is the only
      opportunity to customize it.
    </p>

    <p v-else-if="!supportsCustomisingKernelArgs">
      Please note that kernel command line customization is only accepted for the initial boot
      images (ISO, PXE, disk image) and is ignored for
      <code>installer</code>
      images. For upgrade/install command line customization, use the
      <code>machine.install.extraKernelArgs</code>
      machine configuration field.
    </p>

    <p>Skip this step if unsure.</p>

    <template v-if="selectedSBC">
      <TextArea
        v-model="formState.overlayOptions"
        placeholder="configTxtAppend: 'dtoverlay=vc4-fkms-v3d'"
        title="Extra overlay options (advanced)"
        overhead-title
      />

      <p>
        This step allows you to customize the overlay options for the
        <a
          target="_blank"
          rel="noopener noreferrer"
          :href="`https://github.com/${selectedSBC.spec.overlay_image}`"
          class="link-primary"
        >
          {{ selectedSBC.spec.overlay_image }} overlay.
        </a>
      </p>

      <p>
        The accepted syntax is YAML (JSON) key-value pairs. The available options are specific to
        the overlay image and can be found in its documentation.
      </p>

      <p>If unsure, you can skip this step.</p>
    </template>

    <RadioGroup
      v-if="supportsBootloaderSelection"
      v-model="formState.bootloader"
      label="Bootloader"
    >
      <RadioGroupOption :value="SchematicBootloader.BOOT_AUTO">
        auto

        <template #description>Automatic bootloader selection.</template>
      </RadioGroupOption>

      <RadioGroupOption :value="SchematicBootloader.BOOT_SD">
        sd-boot

        <template #description>Use systemd-boot as bootloader if supported.</template>
      </RadioGroupOption>

      <RadioGroupOption v-if="!formState.secureBoot" :value="SchematicBootloader.BOOT_GRUB">
        grub

        <template #description>Use GRUB as a bootloader (not supported with SecureBoot).</template>
      </RadioGroupOption>

      <RadioGroupOption :value="SchematicBootloader.BOOT_DUAL">
        dual-boot

        <template #description>Use GRUB for BIOS and systemd-boot for UEFI systems.</template>
      </RadioGroupOption>
    </RadioGroup>
  </div>
</template>
