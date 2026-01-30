<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useEventListener } from '@vueuse/core'
import { gte } from 'semver'
import { computed, useTemplateRef } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecBootMethod,
  type SBCConfigSpec,
} from '@/api/omni/specs/virtual.pb'
import {
  CloudPlatformConfigType,
  MetalPlatformConfigType,
  PlatformMetalID,
  SBCConfigType,
  VirtualNamespace,
} from '@/api/resources'
import CodeBlock from '@/components/common/CodeBlock/CodeBlock.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink, getLegacyDocsLink, majorMinorVersion } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useDownloadImage } from '@/methods/useDownloadImage'
import { useResourceGet } from '@/methods/useResourceGet'
import { useTalosctlDownloads } from '@/methods/useTalosctlDownloads'
import { showError } from '@/notification'
import { formStateToPreset } from '@/views/omni/InstallationMedia/formStateToPreset'
import type { FormState } from '@/views/omni/InstallationMedia/useFormState'
import { usePresetDownloadLinks } from '@/views/omni/InstallationMedia/usePresetDownloadLinks'
import { usePresetSchematic } from '@/views/omni/InstallationMedia/usePresetSchematic'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

defineProps<{
  isReviewPage?: boolean
}>()

const formState = defineModel<FormState>({ required: true })

const supportsUnifiedInstaller = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.10.0'),
)
const talosctlAvailable = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.11.0-alpha.3'),
)

const { data: features } = useFeatures()
const imageDownloadDialog = useTemplateRef<HTMLDialogElement>('downloadImageDialog')
const {
  isGenerating: imageIsGenerating,
  abort: abortImageDownload,
  download: _downloadImage,
} = useDownloadImage()

async function downloadImage(url: string) {
  try {
    imageDownloadDialog.value?.showModal()
    await _downloadImage(url)
  } catch (error) {
    showError('Image download failed', error instanceof Error ? error.message : String(error))
  } finally {
    imageDownloadDialog.value?.close()
  }
}

useEventListener(imageDownloadDialog, 'close', abortImageDownload)

const { downloads } = useTalosctlDownloads()

const talosctlPaths = computed(
  () => downloads.value?.get(`v${formState.value.talosVersion}`)?.map((v) => v.url) ?? [],
)

const { data: selectedCloudProvider } = useResourceGet<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'cloud',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: CloudPlatformConfigType,
    id: formState.value.cloudPlatform,
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

const { data: metalProvider } = useResourceGet<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'metal',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: MetalPlatformConfigType,
    id: PlatformMetalID,
  },
}))

const selectedPlatform = computed(() =>
  formState.value.hardwareType === 'metal' ? metalProvider.value : selectedCloudProvider.value,
)

const secureBootSuffix = computed(() => {
  if (!formState.value.secureBoot) {
    return ''
  }

  return '-secureboot'
})

// true if the platform supports boot methods other than disk-image.
const notOnlyDiskImage = computed(
  () =>
    selectedPlatform.value?.spec.boot_methods?.some(
      (m) => m !== PlatformConfigSpecBootMethod.DISK_IMAGE,
    ) ?? false,
)

const preset = computed(() => formStateToPreset(formState.value))

const { schematic, schematicLoading, schematicError } = usePresetSchematic(preset)
const schematicId = computed(() => schematic.value?.id ?? '')

const { links } = usePresetDownloadLinks(schematicId, preset)

const factoryUrl = computed(() => features.value?.spec.image_factory_base_url)

const installerImage = computed(() =>
  supportsUnifiedInstaller.value
    ? `${factoryUrl.value}/${formState.value.hardwareType}-installer${secureBootSuffix.value}/${schematicId.value}:${formState.value.talosVersion}`
    : `${factoryUrl.value}/installer/${schematicId.value}:${formState.value.talosVersion}`,
)
</script>

<template>
  <div v-if="schematic" class="flex flex-col gap-4 text-xs">
    <h2 v-if="!isReviewPage" class="text-sm text-naturals-n14">Schematic Ready</h2>

    <p class="flex items-center gap-1">
      Your image schematic ID is:
      <code class="rounded bg-naturals-n4 px-2 py-1 wrap-anywhere">{{ schematic.id }}</code>
      <CopyButton :text="schematic.id" />
    </p>

    <CodeBlock :code="schematic.yml" />

    <h3 class="text-sm text-naturals-n14">First Boot</h3>
    <p v-if="formState.hardwareType === 'metal'">
      Here are the options for the initial boot of Talos Linux on a bare-metal machine or a generic
      virtual machine:
    </p>
    <p v-else-if="selectedPlatform">
      Here are the options for the initial boot of Talos Linux on
      {{ selectedPlatform.spec.label }}:
    </p>
    <p v-else-if="selectedSBC">Use the following disk image for {{ selectedSBC.spec.label }}:</p>

    <dl class="flex flex-col gap-2">
      <template v-for="{ label, link, documentation, copyOnly } in links" :key="link">
        <dt class="font-medium text-naturals-n14 not-first-of-type:mt-2">
          {{ label }}
          <Tooltip v-if="documentation" description="Documentation">
            <a
              class="link-primary align-bottom"
              :href="documentation.link"
              target="_blank"
              rel="noopener noreferrer"
            >
              <TIcon icon="documentation" :aria-label="documentation.label" class="size-4" />
            </a>
          </Tooltip>
        </dt>

        <dd v-if="copyOnly" class="flex items-center gap-1">
          <code class="whitespace-wrap rounded bg-naturals-n4 px-2 py-1 wrap-anywhere">
            {{ link }}
          </code>
          <CopyButton :text="link" />
        </dd>

        <dd v-else>
          <a
            class="link-primary"
            :href="link"
            target="_blank"
            rel="noopener noreferrer"
            @click.prevent="downloadImage(link)"
          >
            {{ link }}
          </a>
        </dd>
      </template>
    </dl>

    <template v-if="notOnlyDiskImage">
      <h3 class="text-sm text-naturals-n14">Initial Installation</h3>
      <p>
        For the initial installation of Talos Linux (not applicable for disk image boot), add the
        following installer image to the machine configuration:
      </p>

      <CodeBlock :code="installerImage" />
    </template>

    <template v-if="formState.talosVersion && gte(formState.talosVersion, '1.12.0-alpha.2')">
      <h3 class="text-sm text-naturals-n14">Local Test Cluster</h3>
      <p>
        To create a local Talos Linux test cluster from this schematic on macOS (Apple Silicon) or
        Linux, run:
      </p>
      <CodeBlock
        :code="`talosctl cluster create qemu --schematic-id=${schematic.id} --talos-version=v${formState.talosVersion}`"
      />
    </template>

    <h3 class="text-sm text-naturals-n14">PXE Boot with booter</h3>
    <p>
      To easily PXE boot bare-metal machines using
      <a
        class="link-primary"
        href="https://github.com/siderolabs/booter"
        target="_blank"
        rel="noopener noreferrer"
      >
        booter
      </a>
      with this schematic, run the following command on a host in the same subnet:
    </p>
    <CodeBlock
      :code="`docker run --rm --network host ghcr.io/siderolabs/booter:v0.3.0 --talos-version=v${formState.talosVersion} --schematic-id=${schematic.id}`"
    />

    <h3 class="text-sm text-naturals-n14">Documentation</h3>
    <ul class="ml-2 flex list-inside list-disc flex-col gap-2 text-primary-p3">
      <li>
        <a
          v-if="formState.talosVersion"
          class="link-primary"
          :href="
            getDocsLink('talos', `/getting-started/what's-new-in-talos`, {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          What's New in Talos {{ majorMinorVersion(formState.talosVersion) }}
        </a>
      </li>
      <li>
        <a
          v-if="formState.talosVersion"
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/support-matrix', {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Support Matrix for {{ majorMinorVersion(formState.talosVersion) }}
        </a>
      </li>
      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/getting-started', {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Getting Started Guide
        </a>
      </li>

      <li v-if="formState.hardwareType === 'metal'">
        <a
          class="link-primary"
          :href="
            getDocsLink(
              'talos',
              '/platform-specific-installations/bare-metal-platforms/network-config',
              { talosVersion: formState.talosVersion! },
            )
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Bare-metal Network Configuration
        </a>
      </li>
      <li v-else-if="selectedPlatform">
        <a
          class="link-primary"
          :href="
            getLegacyDocsLink(selectedPlatform.spec.documentation, {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ selectedPlatform.spec.label }} Guide
        </a>
      </li>
      <li v-else-if="selectedSBC">
        <a
          class="link-primary"
          :href="
            getLegacyDocsLink(selectedSBC.spec.documentation, {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ selectedSBC.spec.label }} Guide
        </a>
      </li>

      <li v-if="formState.secureBoot">
        <a
          class="link-primary"
          :href="
            getDocsLink(
              'talos',
              '/platform-specific-installations/bare-metal-platforms/secureboot',
              { talosVersion: formState.talosVersion! },
            )
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          SecureBoot Guide
        </a>
      </li>

      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/prodnotes', {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Production Cluster Guide
        </a>
      </li>

      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/troubleshooting/troubleshooting', {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Troubleshooting Guide
        </a>
      </li>
    </ul>

    <template
      v-if="formState.hardwareType === 'metal' && !formState.secureBoot && talosctlAvailable"
    >
      <h3 class="text-sm text-naturals-n14">Extra Assets</h3>
      <dl class="flex flex-col gap-2 [&_dd+dt]:mt-2 [&_dt]:font-medium [&_dt]:text-naturals-n14">
        <dt>Talosctl CLI</dt>
        <dd v-for="path in talosctlPaths" :key="path">
          <a class="link-primary" :href="path" target="_blank" rel="noopener noreferrer">
            {{ path }}
          </a>
        </dd>
      </dl>
    </template>

    <dialog
      ref="downloadImageDialog"
      closedby="any"
      class="modal-window fixed inset-0 m-auto gap-2 not-open:hidden open:backdrop:bg-naturals-n0/90"
    >
      <div class="mb-5 flex items-center justify-between text-xl text-naturals-n14">
        <h3 class="text-base text-naturals-n14">Image download</h3>

        <CloseButton @click="abortImageDownload" />
      </div>

      <p class="flex items-center gap-1.5">
        <TSpinner class="size-3" />

        <span v-if="imageIsGenerating">Generating ...</span>
      </p>
    </dialog>
  </div>
  <div v-else class="flex flex-col gap-4">
    <p v-if="schematicLoading" class="flex items-center gap-2 text-sm">
      <TSpinner class="size-4" />
      Generating schematic ...
    </p>

    <TAlert v-if="schematicError" title="Failed to create schematic" type="error">
      {{ schematicError }}
    </TAlert>
  </div>
</template>
