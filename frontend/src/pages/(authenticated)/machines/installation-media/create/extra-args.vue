<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { gte } from 'semver'
import { computed, onBeforeMount, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { SchematicBootloader } from '@/api/omni/management/management.pb'
import type { QuirksSpec, SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { QuirksType, SBCConfigType, VirtualNamespace } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import Expandable from '@/components/Expandable/Expandable.vue'
import RadioGroup from '@/components/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/Radio/RadioGroupOption.vue'
import TextArea from '@/components/TextArea/TextArea.vue'
import TInput from '@/components/TInput/TInput.vue'
import { getDocsLink } from '@/methods'
import { useResourceGet } from '@/methods/useResourceGet'
import { type FormState, resolveTalosVersion } from '@/views/InstallationMedia/useFormState'

definePage({ name: 'InstallationMediaCreateExtraArgs' })

const formState = defineModel<FormState>({ required: true })

const resolvedVersion = computed(() => resolveTalosVersion(formState.value.talosVersion!))

const configEditorExpanded = ref(false)

const supportsCustomisingKernelArgs = computed(() => gte(resolvedVersion.value, '1.10.0'))
const supportsBootloaderSelection = computed(() => gte(resolvedVersion.value, '1.12.0-alpha.2'))

const { data: quirks } = useResourceGet<QuirksSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: QuirksType,
    id: resolvedVersion.value,
  },
}))

const { data: selectedSBC } = useResourceGet<SBCConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'sbc',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
    id: formState.value.sbcType,
  },
}))

// Form defaults
onBeforeMount(() => (formState.value.bootloader ??= SchematicBootloader.BOOT_AUTO))
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
        :href="getDocsLink('talos', '/reference/kernel', { talosVersion: resolvedVersion })"
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

    <template v-if="quirks?.spec.supports_embedded_config">
      <div class="flex items-center gap-2">
        <span class="text-sm font-medium text-naturals-n14">Embedded machine configuration</span>

        <IconButton
          icon="fullscreen"
          class="size-6 shrink-0"
          aria-label="Expand editor"
          @click="configEditorExpanded = true"
        />
      </div>

      <Expandable
        v-model:expanded="configEditorExpanded"
        title="Embedded machine configuration"
        class="data-[state=closed]:h-50 data-[state=closed]:overflow-hidden data-[state=closed]:rounded data-[state=closed]:border data-[state=closed]:border-naturals-n8"
      >
        <CodeEditor
          v-model="formState.embeddedMachineConfig"
          :talos-version="resolvedVersion"
          :options="{ ariaLabel: 'Embedded machine configuration' }"
          class="size-full"
        />
      </Expandable>

      <p>
        This configuration will be embedded into the image. For more details see the
        <a
          target="_blank"
          rel="noopener noreferrer"
          :href="
            getDocsLink(
              'talos',
              '/configure-your-talos-cluster/system-configuration/acquire#embedded-configuration',
              { talosVersion: resolvedVersion },
            )
          "
          class="link-primary"
        >
          documentation.
        </a>
      </p>

      <p>Skip this step if unsure.</p>
    </template>

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
        Auto

        <template #description>
          Automatically select bootloader support based on image type and architecture
          (recommended).
        </template>
      </RadioGroupOption>

      <RadioGroupOption :value="SchematicBootloader.BOOT_SD">
        UEFI only

        <template #description>
          Supports modern hardware with UEFI firmware (uses systemd-boot).
        </template>
      </RadioGroupOption>

      <RadioGroupOption v-if="!formState.secureBoot" :value="SchematicBootloader.BOOT_GRUB">
        BIOS only

        <template #description>
          Legacy hardware support, some virtual machines (uses GRUB).
        </template>
      </RadioGroupOption>

      <RadioGroupOption :value="SchematicBootloader.BOOT_DUAL">
        Dual Boot

        <template #description>
          Universal image, includes support for BIOS and UEFI systems.
        </template>
      </RadioGroupOption>
    </RadioGroup>
  </div>
</template>
