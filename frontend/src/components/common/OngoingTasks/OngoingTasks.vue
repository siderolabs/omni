<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource } from '@/api/grpc'
import { KubernetesUpgradeStatusSpecPhase, type OngoingTaskSpec } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'
import { itemID } from '@/api/watch'
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import IconHeaderDropdownLoading from '@/components/icons/IconHeaderDropdownLoading.vue'
import { formatISO } from '@/methods/time'

const { data } = useWatch<OngoingTaskSpec>(() => ({
  resource: {
    namespace: EphemeralNamespace,
    type: OngoingTaskType,
  },
  runtime: Runtime.Omni,
}))

const dropdownOpen = ref(false)

const getPreviousVersion = (item: Resource<OngoingTaskSpec>) => {
  if (item.spec.kubernetes_upgrade) {
    return item.spec.kubernetes_upgrade.last_upgrade_version
  }

  if (item.spec.talos_upgrade) {
    return item.spec.talos_upgrade.last_upgrade_version
  }

  return item.spec.machine_upgrade?.current_schematic_id
}

const getCurrentVersion = (item: Resource<OngoingTaskSpec>) => {
  if (item.spec.kubernetes_upgrade) {
    return item.spec.kubernetes_upgrade.current_upgrade_version
  }

  if (item.spec.talos_upgrade) {
    return item.spec.talos_upgrade.current_upgrade_version
  }

  return item.spec.machine_upgrade?.schematic_id
}

const toggleDropdown = () => {
  dropdownOpen.value = !dropdownOpen.value
}

const onClickOutside = () => {
  dropdownOpen.value = false
}
</script>

<template>
  <div v-on-click-outside="onClickOutside" class="relative flex items-center">
    <div
      class="flex cursor-pointer items-center gap-1 text-naturals-n11 transition-colors hover:text-naturals-n14"
      @click="() => toggleDropdown()"
    >
      <IconHeaderDropdownLoading :active="data.length > 0" />
      <span class="text-xs font-normal whitespace-nowrap select-none">
        <span class="contents sm:hidden">Tasks</span>
        <span class="hidden sm:contents">Ongoing Tasks</span>
      </span>
      <TIcon
        class="flex h-4 w-4 items-center justify-center fill-current transition-transform duration-300"
        :class="{ '-rotate-180': dropdownOpen }"
        icon="arrow-down"
      />
    </div>

    <TAnimation>
      <div
        v-show="dropdownOpen"
        class="absolute top-7 -right-2 z-30 rounded border border-naturals-n4 bg-naturals-n2"
      >
        <div
          v-for="item in data"
          :key="itemID(item)"
          class="flex flex-col gap-2 p-6 not-last:border-b not-last:border-naturals-n4"
        >
          <div class="flex items-center justify-between gap-4">
            <h3 class="w-9/12 text-xs whitespace-nowrap text-naturals-n13">
              {{ item.spec.title }}
            </h3>

            <span v-if="item.metadata.created" class="w-3/12 text-right text-xs text-naturals-n9">
              {{ formatISO(item.metadata.created, 'HH:mm:ss') }}
            </span>
          </div>

          <div class="truncate text-xs text-naturals-n9">
            <template v-if="item.spec.destroy">
              {{ item.spec.destroy.phase }}
            </template>

            <div v-else class="flex items-center gap-2 text-xs">
              <template v-if="getCurrentVersion(item)">
                <span v-if="item.spec.kubernetes_upgrade">Upgrade Kubernetes</span>
                <span v-else-if="item.spec.machine_upgrade">Upgrade Machine</span>
                <span v-else>Upgrade Talos</span>
                <div class="flex-1" />
                <span class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13">
                  {{ getPreviousVersion(item) }}
                </span>
                <span>⇾</span>
                <span class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13">
                  {{ getCurrentVersion(item) }}
                </span>
              </template>

              <template
                v-else-if="
                  (item.spec?.kubernetes_upgrade?.phase ?? item.spec?.talos_upgrade?.phase) ===
                  KubernetesUpgradeStatusSpecPhase.Reverting
                "
              >
                <span>Reverting back to</span>
                <div class="flex-1" />
                <span class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13">
                  {{
                    (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)?.last_upgrade_version
                  }}
                </span>
              </template>

              <span v-else>Updating machine schematics</span>
            </div>
          </div>
        </div>

        <div v-if="!data.length" class="flex w-32 items-center justify-center gap-2 p-4 text-xs">
          <TIcon icon="check" class="h-4 w-4" />
          No tasks
        </div>
      </div>
    </TAnimation>
  </div>
</template>
