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
  <div v-click-outside="onClickOutside" class="dropdown-wrapper">
    <Watch v-if="watchOpts" :opts="watchOpts" display-always>
      <template #default="{ items }">
        <div class="dropdown" @click="() => toggleDropdown()">
          <IconHeaderDropdownLoading :active="items.length > 0" />
          <span class="dropdown-name">Ongoing Tasks</span>
          <TIcon
            class="dropdown-icon"
            :class="{ 'dropdown-icon-open': dropdownOpen }"
            icon="arrow-down"
          />
        </div>
        <TAnimation>
          <div v-show="dropdownOpen" class="dropdown-list">
            <template v-if="items.length > 0">
              <div v-for="item in items" :key="itemID(item)" class="item">
                <div class="item-heading">
                  <h3 class="item-title">{{ item.spec.title }}</h3>
                  <span class="item-time">{{
                    formatISO(item.metadata.created as string, 'HH:mm:ss')
                  }}</span>
                </div>
                <p class="item-description truncate">
                  <template v-if="item.spec.destroy">
                    {{ item.spec.destroy.phase }}
                  </template>
                  <template v-else>
                    <div class="flex items-center gap-2 text-xs">
                      <template
                        v-if="
                          (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                            .current_upgrade_version
                        "
                      >
                        <span v-if="item.spec.kubernetes_upgrade">Upgrade Kubernetes</span>
                        <span v-else>Upgrade Talos</span>
                        <div class="flex-1" />
                        <span
                          class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13"
                        >
                          {{
                            (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                              .last_upgrade_version
                          }}
                        </span>
                        <span>⇾</span>
                        <span
                          class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13"
                        >
                          {{
                            (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                              .current_upgrade_version
                          }}
                        </span>
                      </template>
                      <template
                        v-else-if="
                          (item.spec?.kubernetes_upgrade?.phase ??
                            item.spec?.talos_upgrade?.phase) ===
                          KubernetesUpgradeStatusSpecPhase.Reverting
                        "
                      >
                        <span>Reverting back to</span>
                        <div class="flex-1" />
                        <span
                          class="rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13"
                        >
                          {{
                            (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)
                              .last_upgrade_version
                          }}
                        </span>
                      </template>
                      <template v-else>
                        <span>Installing extensions</span>
                      </template>
                    </div>
                  </template>
                </p>
              </div>
            </template>
            <div v-else class="flex w-32 items-center justify-center gap-2 p-4 text-xs">
              <TIcon icon="check" class="h-4 w-4" />
              No tasks
            </div>
          </div>
        </TAnimation>
      </template>
    </Watch>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.dropdown {
  @apply flex cursor-pointer items-center gap-1 text-naturals-n11 transition-colors hover:text-naturals-n14;
}
.dropdown-wrapper {
  @apply relative flex items-center;
}
.dropdown-icon {
  @apply flex h-4 w-4 items-center justify-center fill-current transition-transform duration-300;
}
.dropdown-icon-open {
  transform: rotateX(-180deg);
}
.dropdown-name {
  @apply text-xs font-normal whitespace-nowrap select-none;
}
.dropdown-list {
  @apply absolute top-7 -right-2 z-30 rounded border border-naturals-n4 bg-naturals-n2;
}

.item {
  @apply flex flex-col gap-2 p-6;
}

.item:not(:last-child) {
  @apply border-b border-naturals-n4;
}

.item-heading {
  @apply flex items-center justify-between gap-4;
}
.item-title {
  @apply w-9/12 text-xs whitespace-nowrap text-naturals-n13;
}
.item-time {
  @apply w-3/12 text-right text-xs text-naturals-n9;
}
.item-description {
  @apply text-xs text-naturals-n9;
}
</style>
