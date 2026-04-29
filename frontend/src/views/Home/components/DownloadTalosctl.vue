<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import { compare } from 'semver'
import { computed, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { QuirksSpec } from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  QuirksType,
  TalosVersionType,
  VirtualNamespace,
} from '@/api/resources'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink, getPlatform } from '@/methods'
import { useResourceList } from '@/methods/useResourceList'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { useTalosctlDownloads } from '@/methods/useTalosctlDownloads'

const open = defineModel<boolean>('open', { default: false })

const platform = computedAsync(getPlatform)

const selectedVersion = ref<string>()
const selectedBinary = ref<string>()

const { data: quirks } = useResourceList<QuirksSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    type: QuirksType,
    namespace: VirtualNamespace,
  },
}))

const {
  data: versions,
  loading: versionsLoading,
  err: versionsErr,
} = useResourceWatch<TalosVersionSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
}))

const {
  data: binaries,
  loading: binariesLoading,
  err: binariesErr,
} = useTalosctlDownloads(selectedVersion, () => ({ skip: !open.value }))

function getBinaryNameFromURL(url: string) {
  return new URL(url).pathname.split('/').pop()
}

const versionsList = computed(() =>
  versions.value
    .filter(
      (v) =>
        !v.spec.deprecated &&
        quirks.value?.find((q) => q.metadata.id === v.spec.version)?.spec.supports_factory_talosctl,
    )
    .map((v) => v.spec.version!)
    .sort(compare),
)

const binariesList = computed(() =>
  binaries.value.map((b) => ({
    label: getBinaryNameFromURL(b) ?? b,
    value: b,
  })),
)

watchEffect(() => {
  if (!open.value) {
    selectedVersion.value = undefined
    selectedBinary.value = undefined
  }

  const selected = toValue(selectedBinary.value)

  if (selected && !binariesList.value.some((b) => b.value === selected)) {
    selectedBinary.value = binariesList.value.find(
      (b) => getBinaryNameFromURL(b.value) === getBinaryNameFromURL(selected),
    )?.value
  }
})

const defaultBinary = computed(() => {
  if (!binaries.value || !platform.value) return

  const [os, arch] = platform.value

  const ext = os === 'windows' ? '.exe' : ''

  const defaultAsset = binaries.value.find((item) => item.endsWith('linux-amd64'))
  const preferredAsset = binaries.value.find((item) => item.endsWith(`${os}-${arch}${ext}`))

  return preferredAsset ?? defaultAsset
})
</script>

<template>
  <Modal
    v-model:open="open"
    title="Download Talosctl"
    action-label="Download"
    :action-disabled="!selectedBinary"
    :action-href="selectedBinary ?? ''"
    :loading="binariesLoading"
    @confirm="open = false"
  >
    <template #description>
      <code>talosctl</code>
      can be used to access cluster nodes using Talos machine API. Read the
      <a
        class="link-primary"
        target="_blank"
        rel="noopener noreferrer"
        :href="getDocsLink('omni', '/getting-started/how-to-install-talosctl')"
      >
        docs
      </a>
      for more information.
    </template>

    <div class="mb-5 flex flex-col gap-2">
      <span class="text-xs text-naturals-n14">macOS and Linux (recommended)</span>
      <CodeBlock code="brew install siderolabs/tap/sidero-tools" />
    </div>

    <span class="mb-2 text-xs text-naturals-n14">Manual installation</span>

    <TAlert v-if="versionsErr || binariesErr" title="Failed to get talosctl versions" type="error">
      {{ versionsErr }}
      {{ binariesErr }}
    </TAlert>

    <TAlert
      v-else-if="!binariesLoading && !binariesList.length"
      type="warn"
      title="No binaries found"
    >
      No talosctl binaries were found for version {{ selectedVersion }}
    </TAlert>

    <div class="mt-2 mb-5 flex flex-wrap gap-4">
      <TSelectList
        v-if="!versionsLoading && !versionsErr"
        v-model="selectedVersion"
        title="Talos version"
        :default-value="DefaultTalosVersion"
        :values="versionsList"
      />

      <TSelectList
        v-if="binariesList.length"
        v-model="selectedBinary"
        title="Platform"
        :default-value="defaultBinary"
        :values="binariesList"
      />
    </div>

    <p class="flex text-xs">
      More downloads links can be found&nbsp;
      <a
        target="_blank"
        rel="noopener noreferrer"
        class="link-primary"
        href="https://github.com/siderolabs/talos/releases"
      >
        here
      </a>
      .
    </p>
  </Modal>
</template>
