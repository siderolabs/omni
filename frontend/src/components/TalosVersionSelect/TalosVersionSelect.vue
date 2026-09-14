<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { compare } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { QuirksSpec } from '@/api/omni/specs/virtual.pb'
import { DefaultNamespace, QuirksType, TalosVersionType, VirtualNamespace } from '@/api/resources'
import TSelectList, { type SelectItemType } from '@/components/SelectList/TSelectList.vue'
import { useResourceList } from '@/methods/useResourceList'
import { AUTOMATIC_VERSION } from '@/views/InstallationMedia/useFormState'

const { requiresTalosctlSupport, includeAutomatic } = defineProps<{
  title?: string
  overheadTitle?: boolean
  defaultValue?: string
  requiresTalosctlSupport?: boolean
  includeAutomatic?: boolean
}>()

defineEmits<{
  checkedValue: [value: string]
}>()

const model = defineModel<string>()

const { data: talosVersionList, loading: talosVersionsLoading } = useResourceList<TalosVersionSpec>(
  {
    runtime: Runtime.Omni,
    resource: {
      type: TalosVersionType,
      namespace: DefaultNamespace,
    },
  },
)

const { data: quirks, loading: quirksLoading } = useResourceList<QuirksSpec>(() => ({
  skip: !requiresTalosctlSupport,
  runtime: Runtime.Omni,
  resource: {
    type: QuirksType,
    namespace: VirtualNamespace,
  },
}))

const talosVersionOptions = computed(() => {
  const versions: SelectItemType<string>[] = []

  if (includeAutomatic) {
    versions.push({ label: 'Automatic', value: AUTOMATIC_VERSION })
  }

  return versions.concat(
    talosVersionList.value
      .filter((v) => {
        if (v.spec.deprecated) return false
        if (!requiresTalosctlSupport) return true

        const quirk = quirks.value.find((q) => q.metadata.id === v.spec.version)

        return quirk?.spec.supports_factory_talosctl
      })
      .map(({ spec: { version, unsupported } }) => ({
        label: version!,
        value: version!,
        disabled: unsupported,
        tooltip: unsupported ? `This Omni release does not support Talos ${version}.` : undefined,
      }))
      .sort((a, b) => compare(b.value, a.value)),
  )
})
</script>

<template>
  <TSelectList
    v-model="model"
    :disabled="talosVersionsLoading || quirksLoading"
    :values="talosVersionOptions"
    :default-value
    :title
    :overhead-title
    @checked-value="(v) => $emit('checkedValue', v)"
  />
</template>
