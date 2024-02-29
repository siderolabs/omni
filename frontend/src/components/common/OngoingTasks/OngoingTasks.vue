<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div v-click-outside="onClickOutside" class="dropdown-wrapper">
    <watch v-if="watchOpts" :opts="watchOpts" display-always>
      <template #default="{ items }">
        <div class="dropdown" @click="() => toggleDropdown()">
          <icon-header-dropdown-loading :active="items.length > 0"/>
          <span class="dropdown-name">Ongoing Tasks</span>
          <t-icon class="dropdown-icon" :class="{ 'dropdown-icon-open': dropdownOpen }" icon="arrow-down" />
        </div>
        <t-animation>
          <div v-show="dropdownOpen" class="dropdown-list">
            <template v-if="items.length > 0">
              <div class="item" v-for="item in items" :key="itemID(item)">
                <div class="item-heading">
                  <h3 class="item-title">{{ item.spec.title }}</h3>
                  <span class="item-time">{{
                    time.formatISO(item.metadata.created as string, "HH:mm:ss")
                  }}</span>
                </div>
                <p class="item-description truncate">
                  <template v-if="item.spec.destroy">
                    {{ item.spec.destroy.phase }}
                  </template>
                  <template v-else>
                    <div class="flex gap-2 text-xs items-center">
                      <template v-if="(item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade).current_upgrade_version">
                        <span v-if="item.spec.kubernetes_upgrade">Upgrade Kubernetes</span>
                        <span v-else>Upgrade Talos</span>
                        <div class="flex-1"/>
                        <span class="rounded bg-naturals-N4 px-2 text-xs text-naturals-N13 font-bold">
                          {{ (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade).last_upgrade_version }}
                        </span>
                        <span>⇾</span>
                        <span class="rounded bg-naturals-N4 px-2 text-xs text-naturals-N13 font-bold">
                          {{ (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade).current_upgrade_version }}
                        </span>
                      </template>
                      <template v-else-if="(item.spec?.kubernetes_upgrade?.phase ?? item.spec?.talos_upgrade?.phase) === KubernetesUpgradeStatusSpecPhase.Reverting">
                        <span>Reverting back to</span>
                        <div class="flex-1"/>
                        <span class="rounded bg-naturals-N4 px-2 text-xs text-naturals-N13 font-bold">
                          {{ (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade).last_upgrade_version }}
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
            <div v-else class="text-xs w-32 p-4 flex items-center justify-center gap-2">
              <t-icon icon="check" class="w-4 h-4"/>
              No tasks
            </div>
          </div>
        </t-animation>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

import { itemID } from "@/api/watch";
import { Runtime } from "@/api/common/omni.pb";
import {
  EphemeralNamespace,
  OngoingTaskType,
} from "@/api/resources";
import { time } from "@/methods/time";
import { authorized } from "@/methods/key";
import { KubernetesUpgradeStatusSpecPhase } from "@/api/omni/specs/omni.pb";

import TIcon from "@/components/common/Icon/TIcon.vue";
import TAnimation from "@/components/common/Animation/TAnimation.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import IconHeaderDropdownLoading from "@/components/icons/IconHeaderDropdownLoading.vue";

const watchOpts = computed(() => {
  if (authorized.value) {
    return { resource: resource, runtime: Runtime.Omni };
  }

  return undefined;
});

const dropdownOpen = ref(false);

const resource = {
  namespace: EphemeralNamespace,
  type: OngoingTaskType,
};

const toggleDropdown = () => {
  dropdownOpen.value = !dropdownOpen.value;
};

const onClickOutside = () => {
  dropdownOpen.value = false;
};
</script>

<style scoped>
.dropdown {
  @apply flex items-center cursor-pointer gap-1 text-naturals-N11 hover:text-naturals-N13;
}
.dropdown-wrapper {
  @apply relative flex items-center;
}
.dropdown-icon {
  @apply flex justify-center items-center fill-current transition-transform duration-300 w-4 h-4;
}
.dropdown-icon-open {
  transform: rotateX(-180deg);
}
.dropdown-name {
  @apply text-xs font-normal whitespace-nowrap select-none;
}
.dropdown-list {
  @apply absolute -right-2 top-7 bg-naturals-N2 border border-naturals-N4 rounded z-30;
}

.item {
  @apply p-6 flex flex-col gap-2;
}

.item:not(:last-child) {
  @apply border-b border-naturals-N4;
}

.item-heading {
  @apply flex justify-between items-center gap-4;
}
.item-title {
  @apply text-xs text-naturals-N13 w-9/12  whitespace-nowrap;
}
.item-time {
  @apply text-xs text-naturals-N9 w-3/12 text-right;
}
.item-description {
  @apply text-xs text-naturals-N9;
}
</style>