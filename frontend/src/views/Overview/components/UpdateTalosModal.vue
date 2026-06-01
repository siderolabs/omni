<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { compare } from 'semver'
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosUpgradeStatusType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import { majorMinorVersion } from '@/methods'
import { updateTalos } from '@/methods/cluster'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterName } = defineProps<{
  clusterName: string
}>()

const open = defineModel<boolean>('open', { default: false })
const selectedVersion = ref('')

const { data: status } = useResourceWatch<TalosUpgradeStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: clusterName,
  },
  runtime: Runtime.Omni,
}))

watchEffect(() => {
  if (open.value) selectedVersion.value = status.value?.spec.last_upgrade_version || ''
})

const upgradeVersions = computed(() => {
  if (!status.value?.spec?.upgrade_versions) {
    return []
  }

  const sorted = [
    ...status.value.spec.upgrade_versions,
    status.value.spec.last_upgrade_version ?? '',
  ].sort((a, b) => compare(b, a))

  const result: Record<string, string[]> = {}

  for (const version of sorted) {
    const majorMinor = majorMinorVersion(version)

    if (!result[majorMinor]) {
      result[majorMinor] = []
    }

    result[majorMinor].push(version)
  }

  return result
})

const action = computed(() => {
  if (!status.value) {
    return 'Loading...'
  }

  switch (compare(selectedVersion.value, status.value.spec.last_upgrade_version ?? '')) {
    case 1:
      return 'Upgrade'
    case -1:
      return 'Downgrade'
  }

  return 'Unchanged'
})

const upgradeClick = async () => {
  updateTalos(clusterName, selectedVersion.value)

  open.value = false
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Update Talos"
    :action-label="action"
    :action-disabled="!status || selectedVersion === status?.spec?.last_upgrade_version"
    :loading="!status"
    content-class="flex min-h-0 max-w-xl flex-1 flex-col gap-2"
    @confirm="upgradeClick"
  >
    <div class="shrink-0">
      <ManagedByTemplatesWarning warning-style="popup" />
    </div>

    <RadioGroup
      v-if="status"
      v-model="selectedVersion"
      class="flex max-h-64 min-h-16 flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
    >
      <template v-for="(versions, group) in upgradeVersions" :key="group">
        <RadioGroupLabel as="div" class="w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold">
          {{ group }}
        </RadioGroupLabel>
        <div class="flex flex-col gap-1">
          <RadioGroupOption
            v-for="version in versions"
            :key="version"
            v-slot="{ checked }"
            :value="version"
          >
            <div
              class="tranform transition-color flex cursor-pointer items-center gap-2 px-2 py-1 text-sm hover:bg-naturals-n4"
              :class="{ 'bg-naturals-n4': checked }"
            >
              <TCheckbox
                :model-value="checked"
                class="pointer-events-none"
                @vue:mounted="
                  ($event) =>
                    checked && ($event.el as HTMLElement).scrollIntoView({ block: 'center' })
                "
              />
              {{ version }}
              <span v-if="version === status.spec.last_upgrade_version">(current)</span>
            </div>
          </RadioGroupOption>
        </div>
      </template>
    </RadioGroup>

    <p class="shrink-0 text-xs">
      Changing the Talos version can result in control plane downtime. During this change you will
      be able to cancel the upgrade.
    </p>
    <p class="shrink-0 text-xs">This operation starts immediately.</p>
  </Modal>
</template>
