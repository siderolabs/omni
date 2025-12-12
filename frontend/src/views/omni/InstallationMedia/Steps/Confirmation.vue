<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import { dump } from 'js-yaml'
import { gte, parse } from 'semver'
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import {
  CreateSchematicRequestSiderolinkGRPCTunnelMode,
  ManagementService,
} from '@/api/omni/management/management.pb'
import type { InstallationMediaSpec } from '@/api/omni/specs/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecArch,
  PlatformConfigSpecBootMethod,
  type SBCConfigSpec,
} from '@/api/omni/specs/virtual.pb'
import {
  CloudPlatformConfigType,
  EphemeralNamespace,
  InstallationMediaType,
  LabelsMeta,
  MetalPlatformConfigType,
  SBCConfigType,
  VirtualNamespace,
} from '@/api/resources'
import CodeBlock from '@/components/common/CodeBlock/CodeBlock.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink, getLegacyDocsLink } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceList } from '@/methods/useResourceList'
import { useTalosctlDownloads } from '@/methods/useTalosctlDownloads'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const productionGuideAvailable = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.5.0'),
)
const troubleshootingGuideAvailable = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.6.0'),
)
const supportsOverlay = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.7.0'),
)
const supportsUnifiedInstaller = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.10.0'),
)
const talosctlAvailable = computed(
  () => !!formState.value.talosVersion && gte(formState.value.talosVersion, '1.11.0-alpha.3'),
)

const arch = computed(() => {
  switch (formState.value.machineArch) {
    case PlatformConfigSpecArch.AMD64:
      return 'amd64'
    case PlatformConfigSpecArch.ARM64:
      return 'arm64'
    default:
      return formState.value.hardwareType === 'sbc' ? 'arm64' : undefined
  }
})

const { data: features } = useFeatures()

const { downloads } = useTalosctlDownloads()

const talosctlPaths = computed(
  () => downloads.value?.get(`v${formState.value.talosVersion}`)?.map((v) => v.url) ?? [],
)

const { data: cloudProviders } = useResourceList<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'cloud',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: CloudPlatformConfigType,
  },
}))

const { data: SBCs } = useResourceList<SBCConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'sbc',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
  },
}))

const { data: metalProvider } = useResourceGet<PlatformConfigSpec>(() => ({
  skip: formState.value.hardwareType !== 'metal',
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: MetalPlatformConfigType,
    id: 'metal',
  },
}))

const selectedPlatform = computed(() =>
  formState.value.hardwareType === 'metal'
    ? metalProvider.value
    : cloudProviders.value?.find(
        (provider) => formState.value.cloudPlatform === provider.metadata.id,
      ),
)

const selectedSBC = computed(() =>
  SBCs.value?.find((sbc) => sbc.metadata.id === formState.value.sbcType),
)

const profile = computed(() =>
  selectedSBC.value
    ? selectedSBC.value.metadata.id
    : selectedPlatform.value
      ? selectedPlatform.value.metadata.id
      : undefined,
)

const { data: medias, loading: mediaLoading } = useResourceList<InstallationMediaSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: InstallationMediaType,
    namespace: EphemeralNamespace,
  },
})

const noMediaFound = ref(false)
const schematicLoading = ref(false)
const schematicError = ref<string>()

const schematic = computedAsync(async () => {
  const media = medias.value?.find(
    (m) => m.spec.architecture === arch.value && m.spec.profile === profile.value,
  )

  if (!media) {
    if (!mediaLoading.value) {
      noMediaFound.value = true
    }

    return
  }

  noMediaFound.value = false
  schematicLoading.value = true

  const machineLabels = Object.entries(formState.value.machineUserLabels ?? {}).reduce<
    Record<string, string>
  >(
    (prev, [key, { value }]) => ({
      ...prev,
      [key]: value,
    }),
    {},
  )

  try {
    const { schematic_id, schematic_yml, pxe_url } = await ManagementService.CreateSchematic({
      extensions: formState.value.systemExtensions,
      extra_kernel_args: formState.value.cmdline?.split(' ').filter((item) => item.trim()),
      meta_values: {
        [LabelsMeta]: dump({ machineLabels }),
      },
      media_id: media.metadata.id,
      talos_version: formState.value.talosVersion,
      secure_boot: formState.value.secureBoot,
      siderolink_grpc_tunnel_mode: formState.value.useGrpcTunnel
        ? CreateSchematicRequestSiderolinkGRPCTunnelMode.ENABLED
        : CreateSchematicRequestSiderolinkGRPCTunnelMode.DISABLED,
      join_token: formState.value.joinToken,
    })

    return {
      id: schematic_id,
      yml: schematic_yml,
      pxeUrl: pxe_url,
    }
  } catch (error) {
    schematicError.value = error?.message || String(error)
  } finally {
    schematicLoading.value = false
  }
})

// true if the platform supports boot methods other than disk-image.
const notOnlyDiskImage = computed(
  () =>
    selectedPlatform.value?.spec.boot_methods?.some(
      (m) => m !== PlatformConfigSpecBootMethod.DISK_IMAGE,
    ) ?? false,
)

const factoryUrl = computed(() => features.value?.spec.image_factory_base_url)

const secureBootInstallerImage = computed(() =>
  supportsUnifiedInstaller.value
    ? `${factoryUrl.value}/${formState.value.hardwareType}-installer-secureboot/${schematic.value?.id}:${formState.value.talosVersion}`
    : `${factoryUrl.value}/installer-secureboot/${schematic.value?.id}:${formState.value.talosVersion}`,
)

const installerImage = computed(() =>
  supportsUnifiedInstaller.value
    ? `${factoryUrl.value}/${formState.value.hardwareType}-installer/${schematic.value?.id}:${formState.value.talosVersion}`
    : `${factoryUrl.value}/installer/${schematic.value?.id}:${formState.value.talosVersion}`,
)

const imageBaseURL = computed(
  () => `${factoryUrl.value}/image/${schematic.value?.id}/${formState.value.talosVersion}`,
)

const sbcDiskImagePath = computed(() => {
  if (!selectedSBC.value) return

  const path = supportsOverlay.value
    ? `metal-${arch.value}.raw.xz`
    : `metal-${selectedSBC.value.metadata.id}-${arch.value}.raw.xz`

  return `${imageBaseURL.value}/${path}`
})

const platformDiskImagePath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}.${selectedPlatform.value.spec.disk_image_suffix}`
    : undefined,
)

const platformSecureBootDiskImagePath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}-secureboot.${selectedPlatform.value.spec.disk_image_suffix}`
    : undefined,
)

const qcow2DiskImagePath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}.qcow2`
    : undefined,
)

const isoPath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}.iso`
    : undefined,
)

const isoSecureBootPath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}-secureboot.iso`
    : undefined,
)

const ukiPath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}-uki.efi`
    : undefined,
)

const secureBootUKIPath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}-secureboot-uki.efi`
    : undefined,
)

const kernelPath = computed(() =>
  selectedPlatform.value ? `${imageBaseURL.value}/kernel-${arch.value}` : undefined,
)

const cmdLinePath = computed(() =>
  selectedPlatform.value
    ? `${imageBaseURL.value}/cmdline-${selectedPlatform.value.metadata.id}-${arch.value}`
    : undefined,
)

const initramfsPath = computed(() =>
  selectedPlatform.value ? `${imageBaseURL.value}/initramfs-${arch.value}.xz` : undefined,
)

function shortVersion(version?: string) {
  if (!version) return

  const { major, minor } = parse(version, false, true)

  return `${major}.${minor}`
}
</script>

<template>
  <div v-if="schematic" class="flex flex-col gap-4 text-xs">
    <h2 class="text-sm text-naturals-n14">Schematic Ready</h2>

    <p class="flex items-center gap-1">
      Your image schematic ID is:
      <code class="rounded bg-naturals-n4 px-2 py-1">{{ schematic.id }}</code>
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

    <dl class="flex flex-col gap-2 [&_dd+dt]:mt-2 [&_dt]:font-medium [&_dt]:text-naturals-n14">
      <template v-if="selectedSBC">
        <dt>Disk Image</dt>
        <dd>
          <a
            class="link-primary"
            :href="sbcDiskImagePath"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ sbcDiskImagePath }}
          </a>
        </dd>
      </template>

      <template
        v-for="bootMethod in selectedPlatform.spec.boot_methods"
        v-else-if="selectedPlatform"
        :key="bootMethod"
      >
        <template v-if="bootMethod === PlatformConfigSpecBootMethod.DISK_IMAGE">
          <template v-if="formState.secureBoot">
            <dt>SecureBoot Disk Image</dt>
            <dd>
              <a
                class="link-primary"
                :href="platformSecureBootDiskImagePath"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ platformSecureBootDiskImagePath }}
              </a>

              <span v-if="formState.hardwareType === 'metal'" class="ml-1 inline-flex">
                (
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
                  SecureBoot documentation
                </a>
                )
              </span>
            </dd>
          </template>
          <template v-else>
            <template v-if="formState.hardwareType === 'metal'">
              <dt>Disk Image (raw)</dt>
              <dd>
                <a
                  class="link-primary"
                  :href="platformDiskImagePath"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ platformDiskImagePath }}
                </a>
              </dd>
              <dt>Disk Image (qcow2)</dt>
              <dd>
                <a
                  class="link-primary"
                  :href="qcow2DiskImagePath"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ qcow2DiskImagePath }}
                </a>
              </dd>
            </template>
            <template v-else>
              <dt>Disk Image</dt>
              <dd>
                <a
                  class="link-primary"
                  :href="platformDiskImagePath"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ platformDiskImagePath }}
                </a>
              </dd>
            </template>
          </template>
        </template>
        <template v-else-if="bootMethod === PlatformConfigSpecBootMethod.ISO">
          <template v-if="formState.secureBoot">
            <dt>SecureBoot ISO</dt>
            <dd>
              <a
                class="link-primary"
                :href="isoSecureBootPath"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ isoSecureBootPath }}
              </a>

              <span class="ml-1 inline-flex">
                (
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
                  SecureBoot documentation
                </a>
                )
              </span>
            </dd>
          </template>
          <template v-else>
            <dt>ISO</dt>
            <dd>
              <a class="link-primary" :href="isoPath" target="_blank" rel="noopener noreferrer">
                {{ isoPath }}
              </a>

              <span v-if="formState.hardwareType === 'metal'" class="ml-1 inline-flex">
                (
                <a
                  class="link-primary"
                  :href="
                    getDocsLink(
                      'talos',
                      '/platform-specific-installations/bare-metal-platforms/iso',
                      { talosVersion: formState.talosVersion! },
                    )
                  "
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  ISO documentation
                </a>
                )
              </span>
            </dd>
          </template>
        </template>
        <template v-else-if="bootMethod === PlatformConfigSpecBootMethod.PXE">
          <dt v-if="formState.secureBoot">SecureBoot PXE (iPXE script)</dt>
          <dt v-else>PXE boot (iPXE script)</dt>
          <dd>
            <a
              class="link-primary"
              :href="schematic.pxeUrl"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ schematic.pxeUrl }}
            </a>

            <span v-if="formState.hardwareType === 'metal'" class="ml-1 inline-flex">
              (
              <a
                class="link-primary"
                :href="
                  getDocsLink(
                    'talos',
                    '/platform-specific-installations/bare-metal-platforms/pxe',
                    { talosVersion: formState.talosVersion! },
                  )
                "
                target="_blank"
                rel="noopener noreferrer"
              >
                PXE documentation
              </a>
              )
            </span>
          </dd>
        </template>
      </template>
    </dl>

    <template v-if="notOnlyDiskImage">
      <h3 class="text-sm text-naturals-n14">Initial Installation</h3>
      <p>
        For the initial installation of Talos Linux (not applicable for disk image boot), add the
        following installer image to the machine configuration:
      </p>

      <CodeBlock v-if="formState.secureBoot" :code="secureBootInstallerImage" />
      <CodeBlock v-else :code="installerImage" />
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
          class="link-primary"
          :href="
            getDocsLink('talos', `/getting-started/what's-new-in-talos`, {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          What's New in Talos {{ shortVersion(formState.talosVersion) }}
        </a>
      </li>
      <li>
        <a
          class="link-primary"
          :href="
            getDocsLink('talos', '/getting-started/support-matrix', {
              talosVersion: formState.talosVersion!,
            })
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Support Matrix for {{ shortVersion(formState.talosVersion) }}
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

      <li v-if="productionGuideAvailable">
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

      <li v-if="troubleshootingGuideAvailable">
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

    <template v-if="formState.hardwareType === 'metal'">
      <h3 class="text-sm text-naturals-n14">Extra Assets</h3>
      <dl class="flex flex-col gap-2 [&_dd+dt]:mt-2 [&_dt]:font-medium [&_dt]:text-naturals-n14">
        <template v-if="formState.secureBoot">
          <dt>SecureBoot UKI</dt>
          <dd>
            <a
              class="link-primary"
              :href="secureBootUKIPath"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ secureBootUKIPath }}
            </a>
          </dd>
        </template>
        <template v-else>
          <dt>Kernel Image</dt>
          <dd>
            <a class="link-primary" :href="kernelPath" target="_blank" rel="noopener noreferrer">
              {{ kernelPath }}
            </a>
          </dd>
          <dt>Kernel Command Line</dt>
          <dd>
            <a class="link-primary" :href="cmdLinePath" target="_blank" rel="noopener noreferrer">
              {{ cmdLinePath }}
            </a>
          </dd>
          <dt>Initramfs Image</dt>
          <dd>
            <a class="link-primary" :href="initramfsPath" target="_blank" rel="noopener noreferrer">
              {{ initramfsPath }}
            </a>
          </dd>
          <dt>UKI</dt>
          <dd>
            <a class="link-primary" :href="ukiPath" target="_blank" rel="noopener noreferrer">
              {{ ukiPath }}
            </a>
          </dd>

          <template v-if="talosctlAvailable">
            <dt>Talosctl CLI</dt>
            <dd v-for="path in talosctlPaths" :key="path">
              <a class="link-primary" :href="path" target="_blank" rel="noopener noreferrer">
                {{ path }}
              </a>
            </dd>
          </template>
        </template>
      </dl>
    </template>
  </div>
  <div v-else class="flex flex-col gap-4">
    <p v-if="schematicLoading" class="flex items-center gap-2 text-sm">
      <TSpinner class="size-4" />
      Generating schematic ...
    </p>

    <TAlert v-if="noMediaFound" title="Failed to create schematic" type="error">
      No installation media available for
      <span class="font-medium text-naturals-n14">{{ profile }}-{{ arch }}</span>
    </TAlert>

    <TAlert v-if="schematicError" title="Failed to create schematic" type="error">
      {{ schematicError }}
    </TAlert>
  </div>
</template>
