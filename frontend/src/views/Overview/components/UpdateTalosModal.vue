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
import type { TalosUpgradeStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosUpgradeStatusType, TalosVersionType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import { majorMinorVersion } from '@/methods'
import { updateTalos } from '@/methods/cluster'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

const { clusterName } = defineProps<{
  clusterName: string
}>()

const open = defineModel<boolean>('open', { default: false })
const selectedVersion = ref('')
const updating = ref(false)

const { data: status, loading: statusLoading } = useResourceWatch<TalosUpgradeStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: clusterName,
  },
  runtime: Runtime.Omni,
}))

const { data: allTalosVersions } = useResourceWatch<TalosVersionSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: TalosVersionType,
  },
  runtime: Runtime.Omni,
}))

const versionMap = computed(() => new Map(allTalosVersions.value.map((v) => [v.spec.version!, v])))
const currentVersion = computed(() => status.value?.spec.last_upgrade_version)

watchEffect(() => {
  if (open.value) selectedVersion.value = currentVersion.value || ''
})

interface VersionGroup {
  versions: string[]
  unsupported: boolean
}

const groupedVersions = computed<Record<string, VersionGroup>>(() => {
  if (!status.value?.spec) return {}

  const supportedVersions = new Set(
    [...(status.value.spec.upgrade_versions ?? []), currentVersion.value ?? ''].filter((v) => !!v),
  )

  const sourceVersion = currentVersion.value
    ? versionMap.value.get(currentVersion.value)
    : undefined

  return [
    ...new Set([
      ...supportedVersions,
      ...(sourceVersion?.spec.upgradable_talos_versions ?? []).filter(
        (v) => versionMap.value.get(v)?.spec.unsupported,
      ),
    ]),
  ]
    .sort((a, b) => compare(b, a))
    .reduce<Record<string, VersionGroup>>((prev, version) => {
      const majorMinor = majorMinorVersion(version)

      prev[majorMinor] ||= {
        versions: [],
        unsupported: true,
      }

      if (supportedVersions.has(version)) {
        prev[majorMinor].versions.push(version)
        prev[majorMinor].unsupported = false
      }

      return prev
    }, {})
})

const action = computed(() => {
  if (!status.value) {
    return 'Loading...'
  }

  switch (compare(selectedVersion.value, currentVersion.value ?? '')) {
    case 1:
      return 'Upgrade'
    case -1:
      return 'Downgrade'
  }

  return 'Unchanged'
})

const upgradeClick = async () => {
  updating.value = true

  try {
    await updateTalos(clusterName, selectedVersion.value)

    open.value = false
  } catch (e) {
    showError(e instanceof Error ? e.message : String(e))
  } finally {
    updating.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Update Talos"
    :action-label="action"
    :action-disabled="!status || selectedVersion === currentVersion"
    :loading="statusLoading || updating"
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
      <template v-for="(group, label) in groupedVersions" :key="label">
        <RadioGroupLabel
          as="div"
          class="sticky top-0 w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold"
        >
          {{ label }}{{ group.unsupported ? ' - Not supported by this Omni release' : '' }}
        </RadioGroupLabel>
        <div class="flex flex-col gap-1">
          <RadioGroupOption
            v-for="version in group.versions"
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
              <span v-if="version === currentVersion">(current)</span>
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
