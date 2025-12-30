<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { DocumentArrowDownIcon } from '@heroicons/vue/24/outline'
import { useClipboard } from '@vueuse/core'
import yaml from 'js-yaml'
import * as semver from 'semver'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { CreateSchematicRequest } from '@/api/omni/management/management.pb'
import {
  CreateSchematicRequestSiderolinkGRPCTunnelMode,
  ManagementService,
} from '@/api/omni/management/management.pb'
import type { InstallationMediaSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { DefaultJoinTokenSpec, JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultJoinTokenID,
  DefaultJoinTokenType,
  DefaultNamespace,
  DefaultTalosVersion,
  EphemeralNamespace,
  InstallationMediaType,
  JoinTokenStatusType,
  LabelsMeta,
  SecureBoot,
  TalosVersionType,
} from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import SplitButton from '@/components/common/Button/SplitButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import GrpcTunnelCheckbox from '@/components/common/GrpcTunnelCheckbox/GrpcTunnelCheckbox.vue'
import Labels, { type LabelSelectItem } from '@/components/common/Labels/Labels.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { formatBytes } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import ExtensionsPicker from '@/views/omni/Extensions/ExtensionsPicker.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

enum Phase {
  Idle = 0,
  Generating = 1,
  Loading = 2,
}

const installExtensions = ref<Record<string, boolean>>({})
const router = useRouter()
const route = useRoute()
const { data: features } = useFeatures()
const { copy } = useClipboard()

const phase = ref(Phase.Idle)
const showDescriptions = ref(false)
const fileSizeLoaded = ref(0)
const kernelArguments = ref('')
const creatingSchematic = ref(false)
const joinToken = ref('')

let controller: AbortController
let closed = false

const abortDownload = () => {
  phase.value = Phase.Idle

  if (controller) {
    controller.abort()
  }
}

const close = () => {
  abortDownload()
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const labels = ref<Record<string, LabelSelectItem>>({})

const optionNames = ref<string[]>([])

const options = ref(new Map<string, Resource<InstallationMediaSpec>>())

const defaultValue = ref('')

const { data: watchOptions } = useResourceWatch<InstallationMediaSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: InstallationMediaType,
    namespace: EphemeralNamespace,
  },
})

const { data: talosVersionsResources } = useResourceWatch<TalosVersionSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
})

const { data: joinTokens } = useResourceWatch<JoinTokenStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: JoinTokenStatusType,
    namespace: DefaultNamespace,
  },
})

const schematicID = ref<string>()
const pxeBootUrl = ref<string>()
const secureBoot = ref(false)

const imageProfile = computed(() => {
  if (!installationMedia.value) return

  let profile = installationMedia.value.spec.overlay
    ? 'metal'
    : installationMedia.value.spec.profile

  // legacy SBC support
  if (installationMedia.value.spec.overlay && semver.lt(selectedTalosVersion.value, '1.7.0')) {
    profile += `-${installationMedia.value.spec.profile}`
  }

  profile += `-${installationMedia.value.spec.architecture}`

  if (secureBoot.value && !installationMedia.value.spec.no_secure_boot) {
    profile += '-secureboot'
  }

  return profile
})

const omniDownloadPath = computed(() => {
  if (!schematicID.value || !installationMedia.value?.metadata.id) return

  const url = `/image/${schematicID.value}/v${selectedTalosVersion.value}/${installationMedia.value.metadata.id}`

  if (!secureBoot.value || installationMedia.value.spec.no_secure_boot) {
    return url
  }

  const params = new URLSearchParams({ [SecureBoot]: 'true' })

  return `${url}?${params}`
})

const factoryDownloadPath = computed(() => {
  if (!schematicID.value || !installationMedia.value || !imageProfile.value) return

  return `/image/${schematicID.value}/v${selectedTalosVersion.value}/${imageProfile.value}.${installationMedia.value.spec.extension}`
})

const imageDownloadUrl = computed(() => {
  if (!factoryDownloadPath.value) return
  if (!features.value?.spec.image_factory_base_url) return factoryDownloadPath.value

  return new URL(factoryDownloadPath.value, features.value.spec.image_factory_base_url)
})

const talosVersions = computed(() =>
  talosVersionsResources.value
    ?.filter((res) => !res.spec.deprecated)
    .map((res) => res.metadata.id!)
    .sort(semver.compare),
)

const selectedOption = ref('')
const selectedTalosVersion = ref(DefaultTalosVersion)
const useGrpcTunnel = ref(false)

const joinTokenOptions = computed(() => {
  return joinTokens.value?.map((res) => res.spec.name!)
})

const minTalosVersion = computed(() => {
  const option = options.value.get(selectedOption.value)
  if (!option) {
    return null
  }

  return option.spec.min_talos_version
})

const supported = computed(() => {
  if (minTalosVersion.value === null) {
    return false
  }

  if (!minTalosVersion.value) {
    return true
  }

  const selectedVersion = semver.parse(selectedTalosVersion.value, { loose: true })
  if (!selectedVersion) return false

  selectedVersion.prerelease = []

  if (semver.lt(selectedVersion.format(), minTalosVersion.value, { loose: true })) {
    return false
  }

  return true
})

watch(
  () => watchOptions.value.length,
  () => {
    options.value = watchOptions.value.reduce((map, obj) => {
      return map.set(obj.spec.name!, obj)
    }, options.value)

    optionNames.value = watchOptions.value.map((item) => item.spec.name!).sort()

    defaultValue.value = optionNames.value[0]
    selectedOption.value = defaultValue.value
  },
)

onMounted(async () => {
  if (route.query.joinToken) {
    const token = await ResourceService.Get(
      {
        namespace: DefaultNamespace,
        type: JoinTokenStatusType,
        id: route.query.joinToken as string,
      },
      withRuntime(Runtime.Omni),
    )

    joinToken.value = token.spec.name

    return
  }

  const defaultToken: Resource<DefaultJoinTokenSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: DefaultJoinTokenType,
      id: DefaultJoinTokenID,
    },
    withRuntime(Runtime.Omni),
  )

  const defaultTokenStatus: Resource<JoinTokenStatusSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: JoinTokenStatusType,
      id: defaultToken.spec.token_id!,
    },
    withRuntime(Runtime.Omni),
  )

  joinToken.value = defaultTokenStatus.spec.name!
})

onUnmounted(abortDownload)

const installationMedia = computed(() => options.value.get(selectedOption.value))

const schematicReq = computed(() => {
  const grpcTunnelMode = useGrpcTunnel.value
    ? CreateSchematicRequestSiderolinkGRPCTunnelMode.ENABLED
    : CreateSchematicRequestSiderolinkGRPCTunnelMode.DISABLED

  const token = joinTokens.value.find((item) => item.spec.name === joinToken.value)

  const schematic: CreateSchematicRequest = {
    extensions: [],
    extra_kernel_args: [],
    meta_values: {},
    media_id: installationMedia.value?.metadata.id,
    talos_version: selectedTalosVersion.value,
    secure_boot: secureBoot.value,
    siderolink_grpc_tunnel_mode: grpcTunnelMode,
    join_token: token?.metadata.id,
  }

  if (labels.value && Object.keys(labels.value).length > 0) {
    const l: Record<string, string> = {}
    for (const k in labels.value) {
      l[k] = labels.value[k].value
    }

    schematic.meta_values![LabelsMeta] = yaml.dump({
      machineLabels: l,
    })
  }

  for (const key in installExtensions.value) {
    if (installExtensions.value[key]) {
      schematic.extensions?.push(key)
    }
  }

  schematic.extra_kernel_args = kernelArguments.value.split(' ').filter((item) => item.trim())

  return schematic
})

const createSchematic = async () => {
  if (creatingSchematic.value) {
    return
  }

  creatingSchematic.value = true

  try {
    const resp = await ManagementService.CreateSchematic(schematicReq.value)

    pxeBootUrl.value = resp.pxe_url
    schematicID.value = resp.schematic_id
  } finally {
    creatingSchematic.value = false
  }
}

watch(schematicReq, () => {
  pxeBootUrl.value = undefined
  schematicID.value = undefined
})

const getFilename = (headers: Headers) => {
  const disposition = headers.get('Content-Disposition')
  if (!disposition) {
    throw new Error('no filename header in the response')
  }

  const parts = disposition.split(';')

  return parts[1].split('=')[1]
}

const download = async () => {
  abortDownload()

  controller = new AbortController()

  if (!installationMedia.value) {
    return
  }

  const doRequest = async (...[url, init]: Parameters<typeof fetch>) => {
    const resp = await fetch(url, init)

    if (!resp.ok) {
      throw new Error(`request failed: ${resp.status} ${await resp.text()}`)
    }

    return resp
  }

  try {
    await createSchematic()
    if (!omniDownloadPath.value) throw new Error('Download URL not found')

    phase.value = Phase.Generating

    await doRequest(omniDownloadPath.value, {
      signal: controller.signal,
      method: 'HEAD',
      headers: new Headers({ 'Cache-Control': 'no-store' }),
    })

    phase.value = Phase.Loading

    const resp = await doRequest(omniDownloadPath.value, { signal: controller.signal })

    fileSizeLoaded.value = 0

    const filename = getFilename(resp.headers)

    const res = new Response(
      new ReadableStream({
        async start(controller) {
          const reader = resp.body!.getReader()

          for (;;) {
            const { done, value } = await reader.read()
            if (done) {
              break
            }

            fileSizeLoaded.value += value.byteLength
            controller.enqueue(value)
          }

          controller.close()
        },
      }),
    )

    const a = document.createElement('a')
    const objectURL = window.URL.createObjectURL(await res.blob())
    a.style.display = 'none'
    a.href = objectURL
    a.download = filename
    document.body.appendChild(a)
    a.click()
    window.URL.revokeObjectURL(objectURL)
    a.remove()

    close()
  } catch (e) {
    showError('Download Failed', e.message)

    throw e
  } finally {
    phase.value = Phase.Idle
  }
}

const copyLink = async () => {
  try {
    await createSchematic()
    if (!imageDownloadUrl.value) throw new Error('Image download URL not found')

    copy(imageDownloadUrl.value.toString())
    showSuccess('Copied image download URL')
  } catch (e) {
    showError('Generate link failed', e.message)

    throw e
  }
}

const setOption = (value: string) => {
  selectedOption.value = value
}

const setTalosVersion = (value: string) => {
  selectedTalosVersion.value = value
}

const copyPxeBootUrl = async () => {
  if (!pxeBootUrl.value) {
    await createSchematic()
  }

  if (pxeBootUrl.value) copy(pxeBootUrl.value)
  showSuccess('Copied PXE Boot URL')
}

const downloaded = computed(() => {
  return formatBytes(fileSizeLoaded.value)
})
</script>

<template>
  <div class="modal-window flex flex-col gap-4 overflow-y-scroll" style="height: 90%">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Download Installation Media</h3>
      <CloseButton @click="close" />
    </div>

    <div v-if="phase !== Phase.Idle" class="flex flex-col items-center">
      <div class="flex items-center gap-2">
        <DocumentArrowDownIcon class="h-5 w-5" />
        {{ installationMedia?.spec.name }}
      </div>
      <div class="flex items-center gap-2">
        <TSpinner class="h-5 w-5" />
        <span v-if="phase === Phase.Loading">{{ downloaded }}</span>
        <span v-else>Generating Image</span>
      </div>
    </div>
    <template v-else>
      <div class="flex flex-wrap gap-3">
        <div v-if="talosVersions" class="flex flex-wrap gap-4">
          <TSelectList
            title="Talos Version"
            :default-value="DefaultTalosVersion"
            :values="talosVersions"
            :searcheable="true"
            @checked-value="setTalosVersion"
          />
        </div>

        <div v-if="defaultValue" class="flex flex-wrap gap-4">
          <TSelectList
            title="Options"
            :default-value="defaultValue"
            :values="optionNames"
            :searcheable="true"
            @checked-value="setOption"
          />
        </div>

        <div v-if="joinToken" class="flex flex-wrap gap-4">
          <TSelectList
            title="Join Token"
            :default-value="joinToken"
            :values="joinTokenOptions"
            :searcheable="true"
            @checked-value="(value) => (joinToken = value)"
          />
        </div>
      </div>

      <div class="flex">
        <h3 class="flex-1 text-sm text-naturals-n14">Pre-Install Extensions</h3>
        <TCheckbox v-model="showDescriptions" class="col-span-2" label="Show Descriptions" />
      </div>

      <ExtensionsPicker
        v-model="installExtensions"
        :talos-version="selectedTalosVersion"
        class="flex-1"
        :show-descriptions="showDescriptions"
      />

      <h3 class="text-sm text-naturals-n14">Machine User Labels</h3>

      <div class="flex items-center gap-2">
        <Labels v-model="labels" />
      </div>

      <h3 class="text-sm text-naturals-n14">Additional Kernel Arguments</h3>

      <TInput v-model="kernelArguments" />

      <TCheckbox
        v-model="secureBoot"
        label="Secure Boot"
        :disabled="installationMedia?.spec?.no_secure_boot"
      />

      <GrpcTunnelCheckbox v-model="useGrpcTunnel" />

      <h3 class="text-sm text-naturals-n14">PXE Boot URL</h3>

      <div
        class="flex cursor-pointer items-center gap-2 rounded border border-naturals-n8 px-1.5 py-1.5 text-xs"
        :class="{ 'pointer-events-none': !supported }"
      >
        <IconButton
          class="min-w-min"
          icon="refresh"
          :icon-classes="{ 'animate-spin': creatingSchematic }"
          @click="createSchematic"
        />
        <span class="flex-1 break-all" @click="createSchematic">
          {{ pxeBootUrl ? pxeBootUrl : 'Click to generate' }}
        </span>
        <IconButton class="min-w-min" icon="copy" @click="copyPxeBootUrl" />
      </div>

      <div>
        <p v-if="supported" class="text-xs">
          The generated image will include the kernel arguments required to register with Omni
          automatically.
        </p>
        <p v-else class="text-xs text-primary-p2">
          {{ selectedOption }} supports only Talos version >= {{ minTalosVersion }}.
        </p>
      </div>

      <div class="flex justify-end gap-4">
        <TButton class="h-9 w-32" @click="close">Cancel</TButton>
        <SplitButton
          :actions="['Download', 'Copy image download URL']"
          variant="highlighted"
          :disabled="!supported"
          @click="(action) => (action === 'Download' ? download() : copyLink())"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-auto w-1/2 p-8;
}

.heading {
  @apply flex items-center justify-between text-xl text-naturals-n14;
}
</style>
