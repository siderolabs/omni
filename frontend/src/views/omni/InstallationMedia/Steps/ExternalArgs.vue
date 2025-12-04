<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { lt } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { SBCConfigType, VirtualNamespace } from '@/api/resources'
import TInput from '@/components/common/TInput/TInput.vue'
import { getDocsLink } from '@/methods'
import { useResourceList } from '@/methods/useResourceList'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const isPre1_7 = computed(
  () => !!formState.value.talosVersion && lt(formState.value.talosVersion, '1.7.0'),
)

const isPre1_10 = computed(
  () => !!formState.value.talosVersion && lt(formState.value.talosVersion, '1.10.0'),
)

const { data: SBCs } = useResourceList<SBCConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'sbc' || isPre1_7.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
  },
}))

const customisableSBC = computed(() =>
  SBCs.value?.find((sbc) => sbc.metadata.id === formState.value.sbcType),
)
</script>

<template>
  <div class="flex flex-col gap-4 text-xs [&_code]:font-bold">
    <span class="text-sm font-medium text-naturals-n14">Customization</span>

    <TInput
      v-model="formState.selectedCmdline"
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

    <p v-else-if="isPre1_10">
      Please note that kernel command line customization is only accepted for the initial boot
      images (ISO, PXE, disk image) and is ignored for
      <code>installer</code>
      images. For upgrade/install command line customization, use the
      <code>machine.install.extraKernelArgs</code>
      machine configuration field.
    </p>

    <p>Skip this step if unsure.</p>

    <template v-if="customisableSBC">
      <TInput
        v-model="formState.selectedOverlayOptions"
        placeholder="configTxtAppend: 'dtoverlay=vc4-fkms-v3d'"
        title="Extra overlay options (advanced)"
        overhead-title
      />

      <p>
        This step allows you to customize the overlay options for the
        <a
          target="_blank"
          rel="noopener noreferrer"
          :href="`https://github.com/${customisableSBC.spec.overlay_image}`"
          class="link-primary"
        >
          {{ customisableSBC.spec.overlay_image }} overlay.
        </a>
      </p>

      <p>
        The accepted syntax is YAML (JSON) key-value pairs. The available options are specific to
        the overlay image and can be found in its documentation.
      </p>

      <p>If unsure, you can skip this step.</p>
    </template>
  </div>
</template>
