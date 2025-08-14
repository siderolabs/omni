<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { KubernetesUpgradeStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'
import { itemID } from '@/api/watch'
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import IconHeaderDropdownLoading from '@/components/icons/IconHeaderDropdownLoading.vue'
import { authorized } from '@/methods/key'
import { formatISO } from '@/methods/time'

const watchOpts = computed(() => {
  if (authorized.value) {
    return { resource: resource, runtime: Runtime.Omni }
  }

  return undefined
})

const dropdownOpen = ref(false)

const resource = {
  namespace: EphemeralNamespace,
  type: OngoingTaskType,
}

const toggleDropdown = () => {
  dropdownOpen.value = !dropdownOpen.value
}

const onClickOutside = () => {
  dropdownOpen.value = false
}
</script>

<template>
  <div v-click-outside="onClickOutside" class="relative flex items-center">
    <Watch v-if="watchOpts" :opts="watchOpts" display-always>
      <template #default="{ items }">
        <div
          class="flex cursor-pointer items-center gap-1 text-naturals-n11 transition-colors hover:text-naturals-n14"
          @click="() => toggleDropdown()"
        >
          <IconHeaderDropdownLoading :active="items.length > 0" />
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
              v-for="item in items"
              :key="itemID(item)"
              class="flex flex-col gap-2 p-6 not-last:border-b not-last:border-naturals-n4"
            >
              <div class="flex items-center justify-between gap-4">
                <h3 class="w-9/12 text-xs whitespace-nowrap text-naturals-n13">
                  {{ item.spec.title }}
                </h3>

                <span
                  v-if="item.metadata.created"
                  class="w-3/12 text-right text-xs text-naturals-n9"
                >
                  {{ formatISO(item.metadata.created, 'HH:mm:ss') }}
                </span>
              </div>

              <div class="truncate text-xs text-naturals-n9">
                <template v-if="item.spec.destroy">
                  {{ item.spec.destroy.phase }}
                </template>

                <div v-else class="flex items-center gap-2 text-xs">
                  <template
                    v-if="
                      (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                        .current_upgrade_version
                    "
                  >
                    <span v-if="item.spec.kubernetes_upgrade">Upgrade Kubernetes</span>
                    <span v-else>Upgrade Talos</span>
                    <div class="flex-1" />
                    <span class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13">
                      {{
                        (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                          .last_upgrade_version
                      }}
                    </span>
                    <span>⇾</span>
                    <span class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13">
                      {{
                        (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                          .current_upgrade_version
                      }}
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
                        (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                          .last_upgrade_version
                      }}
                    </span>
                  </template>

                  <span v-else>Installing extensions</span>
                </div>
              </div>
            </div>

            <div
              v-if="!items.length"
              class="flex w-32 items-center justify-center gap-2 p-4 text-xs"
            >
              <TIcon icon="check" class="h-4 w-4" />
              No tasks
            </div>
          </div>
        </TAnimation>
      </template>
    </Watch>
  </div>
</template>
