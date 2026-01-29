<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource } from '@/api/grpc'
import {
  KubernetesUpgradeStatusSpecPhase,
  type OngoingTaskSpec,
  SecretRotationSpecComponent,
} from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'
import { itemID } from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import IconHeaderDropdownLoading from '@/components/icons/IconHeaderDropdownLoading.vue'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { data } = useResourceWatch<OngoingTaskSpec>(() => ({
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

  return item.spec.machine_upgrade?.current_schematic_id
}

const getCurrentComponent = (item: Resource<OngoingTaskSpec>) => {
  if (item.spec.secrets_rotation) {
    switch (item.spec.secrets_rotation.component) {
      case SecretRotationSpecComponent.TALOS_CA:
        return 'Talos CA'
      default:
        return
    }
  }
  return
}
</script>

<template>
  <PopoverRoot v-model:open="dropdownOpen">
    <PopoverTrigger
      class="flex items-center gap-1 text-naturals-n11 transition-colors hover:text-naturals-n14"
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
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="z-30 max-h-(--reka-popover-content-available-height) max-w-[min(--spacing(80),var(--reka-popover-content-available-width))] min-w-(--reka-popover-trigger-width) origin-(--reka-popover-content-transform-origin) overflow-auto rounded border border-naturals-n4 bg-naturals-n2 slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        :side-offset="10"
        align="end"
        side="bottom"
      >
        <div
          v-for="item in data"
          :key="itemID(item)"
          class="flex flex-col gap-2 border-naturals-n4 p-6 not-last:border-b"
        >
          <div class="flex items-center justify-between gap-4">
            <h3 class="truncate text-xs text-naturals-n13">
              {{ item.spec.title }}
            </h3>

            <span
              v-if="item.metadata.created"
              class="shrink-0 text-right text-xs whitespace-nowrap text-naturals-n9"
            >
              {{ formatISO(item.metadata.created, 'HH:mm:ss') }}
            </span>
          </div>

          <div class="text-xs text-naturals-n9">
            <span v-if="item.spec.destroy" class="truncate">
              {{ item.spec.destroy.phase }}
            </span>

            <span v-else-if="item.spec.secrets_rotation" class="whitespace-nowrap">
              Rotating {{ getCurrentComponent(item) }}
            </span>

            <div v-else class="flex items-center gap-2 text-xs">
              <template v-if="getCurrentVersion(item)">
                <span v-if="item.spec.kubernetes_upgrade" class="whitespace-nowrap">
                  Upgrade Kubernetes
                </span>
                <span v-else-if="item.spec.machine_upgrade" class="whitespace-nowrap">
                  Upgrade Machine
                </span>
                <span v-else class="whitespace-nowrap">Upgrade Talos</span>
                <div class="flex-1" />
                <span
                  class="truncate rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13"
                >
                  {{ getPreviousVersion(item) }}
                </span>
                <span>⇾</span>
                <span
                  class="truncate rounded bg-naturals-n4 px-2 text-xs font-bold text-naturals-n13"
                >
                  {{ getCurrentVersion(item) }}
                </span>
              </template>

              <template
                v-else-if="
                  (item.spec?.kubernetes_upgrade?.phase ?? item.spec?.talos_upgrade?.phase) ===
                  KubernetesUpgradeStatusSpecPhase.Reverting
                "
              >
                <span class="whitespace-nowrap">Reverting back to</span>
                <div class="flex-1" />
                <span
                  class="rounded bg-naturals-n4 px-2 text-xs font-bold whitespace-nowrap text-naturals-n13"
                >
                  {{
                    (item.spec.kubernetes_upgrade ?? item.spec.talos_upgrade)?.last_upgrade_version
                  }}
                </span>
              </template>

              <span v-else class="whitespace-nowrap">Updating machine schematics</span>
            </div>
          </div>
        </div>

        <div v-if="!data.length" class="flex w-32 items-center justify-center gap-2 p-4 text-xs">
          <TIcon icon="check" class="h-4 w-4" />
          No tasks
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
