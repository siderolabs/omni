<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { gte } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecBootMethod,
  type QuirksSpec,
  type SBCConfigSpec,
} from '@/api/omni/specs/virtual.pb'
import {
  CloudPlatformConfigType,
  DefaultNamespace,
  MetalPlatformConfigType,
  PlatformMetalID,
  QuirksType,
  SBCConfigType,
  TalosVersionType,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import CopyButton from '@/components/CopyButton/CopyButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { getDocsLink, majorMinorVersion } from '@/methods'
import { useImageFactoryAuth, withImageFactoryAuth } from '@/methods/useImageFactoryAuth'
import { useResolvedFactory } from '@/methods/useResolvedFactory'
import { useResourceGet } from '@/methods/useResourceGet'
import { useTalosctlDownloads } from '@/methods/useTalosctlDownloads'
import { formStateToPreset } from '@/views/InstallationMedia/formStateToPreset'
import { type FormState, resolveTalosVersion } from '@/views/InstallationMedia/useFormState'
import { usePresetDownloadLinks } from '@/views/InstallationMedia/usePresetDownloadLinks'
import { usePresetSchematic } from '@/views/InstallationMedia/usePresetSchematic'
import Scan from '@/views/InstallationMedia/vulnerabilities/Scan.vue'

defineProps<{
  isReviewPage?: boolean
}>()

const formState = defineModel<FormState>({ required: true })

const resolvedTalosVersion = computed(() => resolveTalosVersion(formState.value.talosVersion!))

const { data: quirks } = useResourceGet<QuirksSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: QuirksType,
    id: resolvedTalosVersion.value,
  },
}))

const supportsUnifiedInstaller = computed(() => quirks.value?.spec.supports_unified_installer)
const talosctlAvailable = computed(() => quirks.value?.spec.supports_factory_talosctl)

// The selected Talos version records which factory actually serves it (a version may live only on the
// secondary factory). All of its assets - installer image, SBOM, VEX - must go to that factory, and
// its enterprise status decides which enterprise-only assets are available.
const { data: talosVersion } = useResourceGet<TalosVersionSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosVersionType,
    id: resolvedTalosVersion.value,
  },
}))

const { base: factoryBaseURL, credentials: factoryCredentialsRef } = useResolvedFactory(
  () => talosVersion.value?.spec.image_factory_url,
)

const isEnterpriseFactory = computed(() => talosVersion.value?.spec.is_enterprise)

const imageFactoryAuth = useImageFactoryAuth(
  computed(() => talosVersion.value?.spec.image_factory_url),
)

const { data: talosctlPathsRaw } = useTalosctlDownloads(() => resolvedTalosVersion.value)

const talosctlPaths = computed(() =>
  talosctlPathsRaw.value.map((path) => withImageFactoryAuth(path, imageFactoryAuth.value)!),
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
    selectedPlatform.value?.spec.boot_methods
      ?.filter((m) => !isEnterpriseFactory.value || m !== PlatformConfigSpecBootMethod.PXE)
      .some((m) => m !== PlatformConfigSpecBootMethod.DISK_IMAGE) ?? false,
)

const preset = computed(() => formStateToPreset(formState.value))

const resolvedPreset = computed(() => ({
  ...preset.value,
  talos_version: resolvedTalosVersion.value,
}))

const { schematic, schematicLoading, schematicError } = usePresetSchematic(resolvedPreset)
const schematicId = computed(() => schematic.value?.id ?? '')

const { links } = usePresetDownloadLinks(schematicId, resolvedPreset)

const factoryHost = computed(() => (factoryBaseURL.value ? new URL(factoryBaseURL.value).host : ''))

const installerImage = computed(() =>
  supportsUnifiedInstaller.value
    ? `${factoryHost.value}/${formState.value.hardwareType}-installer${secureBootSuffix.value}/${schematicId.value}:${resolvedTalosVersion.value}`
    : `${factoryHost.value}/installer/${schematicId.value}:${resolvedTalosVersion.value}`,
)

const quote = (input: string) => {
  if (/["\s\\]/.test(input) && !/'/.test(input)) {
    return "'" + input.replace(/(['])/g, '\\$1') + "'"
  }

  if (/["'\s]/.test(input)) {
    return '"' + input.replace(/(["\\$`!])/g, '\\$1') + '"'
  }

  return String(input).replace(/([A-Za-z]:)?([#!"$&'()*,:;<=>?@[\\\]^`{|}])/g, '$1\\$2')
}

const clusterCreateCommand = computed(() => {
  const parts = [
    'talosctl cluster create qemu',
    `--image-factory-url=${factoryBaseURL.value}`,
    `--schematic-id=${schematicId.value}`,
    `--talos-version=v${resolvedTalosVersion.value}`,
  ]

  // The local cluster pulls the installer from the factory that serves this version, so it must
  // authenticate with that factory's credentials.
  const { username, password } = factoryCredentialsRef.value ?? {}
  if (username && password) {
    parts.push(`--image-factory-auth=${quote(`${username}:${password}`)}`)
  }

  return parts.join(' ')
})

const SPDXBaseURL = computed(() =>
  factoryBaseURL.value
    ? `${factoryBaseURL.value}/spdx/${schematicId.value}/v${resolvedTalosVersion.value}/amd64`
    : '',
)

const VEXBaseURL = computed(() =>
  factoryBaseURL.value ? `${factoryBaseURL.value}/vex/v${resolvedTalosVersion.value}/vex.json` : '',
)
</script>

<template>
  <div v-if="schematic" class="flex flex-col gap-4 text-xs">
    <Scan
      v-if="isEnterpriseFactory"
      :schematic-id
      :talos-version="resolvedTalosVersion"
      :arch="formState.machineArch!"
    />

    <template v-else>
      <h3 class="text-sm text-naturals-n14">Vulnerability Scan</h3>
      <p>
        Vulnerability scan reports are only available through the Talos Linux Image Factory
        Enterprise.
      </p>
    </template>

    <h2 v-if="!isReviewPage" class="text-sm text-naturals-n14">Schematic Ready</h2>

    <p class="flex items-center gap-1">
      Your image schematic ID is:
      <code class="rounded bg-naturals-n4 px-2 py-1 wrap-anywhere">{{ schematic.id }}</code>
      <CopyButton aria-label="Copy schematic ID" :text="schematic.id" />
    </p>

    <CodeBlock :button-attrs="{ 'aria-label': 'Copy schematic YAML' }" :code="schematic.yml" />

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
      <template
        v-for="{ label, link, documentation, withChecksums, copyOnly } in links"
        :key="link"
      >
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

        <dd class="flex items-center gap-1.5">
          <template v-if="copyOnly">
            <code class="whitespace-wrap rounded bg-naturals-n4 px-2 py-1 wrap-anywhere">
              {{ link }}
            </code>
            <CopyButton :aria-label="`Copy ${label} link`" :text="link" />
          </template>

          <a v-else class="link-primary" :href="link" target="_blank" rel="noopener noreferrer">
            {{ link }}
          </a>

          <template v-if="withChecksums">
            <Tooltip
              :disabled="isEnterpriseFactory"
              description="Checksums are only available through the Talos Linux Enterprise Image Factory."
            >
              <TButton
                is="a"
                :disabled="!isEnterpriseFactory"
                :href="`${link}.sha256`"
                target="_blank"
                rel="noopener noreferrer"
                size="sm"
                :icon="isEnterpriseFactory ? 'arrow-down-tray' : 'locked'"
                icon-position="left"
              >
                sha256
              </TButton>
            </Tooltip>

            <Tooltip
              :disabled="isEnterpriseFactory"
              description="Checksums are only available through the Talos Linux Enterprise Image Factory."
            >
              <TButton
                is="a"
                :disabled="!isEnterpriseFactory"
                :href="`${link}.sha512`"
                target="_blank"
                rel="noopener noreferrer"
                size="sm"
                :icon="isEnterpriseFactory ? 'arrow-down-tray' : 'locked'"
                icon-position="left"
              >
                sha512
              </TButton>
            </Tooltip>
          </template>
        </dd>
      </template>
    </dl>

    <template v-if="notOnlyDiskImage">
      <h3 class="text-sm text-naturals-n14">Initial Installation</h3>
      <p>
        For the initial installation of Talos Linux (not applicable for disk image boot), add the
        following installer image to the machine configuration:
      </p>

      <CodeBlock
        :button-attrs="{ 'aria-label': 'Copy create Talos test cluster command' }"
        :code="installerImage"
      />
    </template>

    <template v-if="gte(resolvedTalosVersion, '1.12.0-alpha.2')">
      <h3 class="text-sm text-naturals-n14">Local Test Cluster</h3>
      <p>
        To create a local Talos Linux test cluster from this schematic on macOS (Apple Silicon) or
        Linux, run:
      </p>
      <CodeBlock
        :button-attrs="{ 'aria-label': 'Copy create Talos test cluster command' }"
        :code="clusterCreateCommand"
      />
    </template>

    <template v-if="!isEnterpriseFactory">
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
        :button-attrs="{ 'aria-label': 'Copy PXE booter docker run command' }"
        :code="`docker run --rm --network host ghcr.io/siderolabs/booter:v0.3.0 --talos-version=v${resolvedTalosVersion} --schematic-id=${schematic.id}`"
      />
    </template>

    <h3 class="text-sm text-naturals-n14">Documentation</h3>
    <ul class="ml-2 flex list-inside list-disc flex-col gap-2 text-primary-p3">
      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', `/getting-started/what's-new-in-talos`, {
              talosVersion: resolvedTalosVersion,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          What's New in Talos {{ majorMinorVersion(resolvedTalosVersion) }}
        </a>
      </li>
      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/support-matrix', {
              talosVersion: resolvedTalosVersion,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Support Matrix for {{ majorMinorVersion(resolvedTalosVersion) }}
        </a>
      </li>
      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/getting-started', {
              talosVersion: resolvedTalosVersion,
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
              { talosVersion: resolvedTalosVersion },
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
            getDocsLink('talos', selectedPlatform.spec.documentation, {
              talosVersion: resolvedTalosVersion,
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
            getDocsLink('talos', selectedSBC.spec.documentation, {
              talosVersion: resolvedTalosVersion,
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
              { talosVersion: resolvedTalosVersion },
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
              talosVersion: resolvedTalosVersion,
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
              talosVersion: resolvedTalosVersion,
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

    <h3 class="text-sm text-naturals-n14">SBOM (SPDX)</h3>
    <a v-if="isEnterpriseFactory" :href="SPDXBaseURL" class="link-primary max-w-max">
      {{ SPDXBaseURL }}
    </a>
    <span v-else>
      SBOM (SPDX) bundle is only available through the Talos Linux Image Factory Enterprise.
    </span>

    <h3 class="text-sm text-naturals-n14">VEX (Vulnerability Exploitability eXchange)</h3>
    <a v-if="isEnterpriseFactory" :href="VEXBaseURL" class="link-primary max-w-max">
      {{ VEXBaseURL }}
    </a>
    <span v-else>
      VEX document is only available through the Talos Linux Image Factory Enterprise.
    </span>
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
