<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import pluralize from 'pluralize'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ExposedServiceSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, ExposedServiceType, LabelCluster } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TMenuItem from '@/components/MenuItem/TMenuItem.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterId } = defineProps<{
  clusterId: string
}>()

const { data: exposedServices } = useResourceWatch<ExposedServiceSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ExposedServiceType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const filteredExposedServices = computed(() => {
  return exposedServices.value.filter((item) => !item.spec.error)
})

const errors = computed(() => {
  const servicesWithErrors = exposedServices.value.filter((item) => item.spec.error)

  return servicesWithErrors.map((item) => item.spec.error)
})
</script>

<template>
  <Disclosure as="div" class="border-t border-border-default" default-open>
    <template #default="{ open }">
      <DisclosureButton as="div" class="disclosure">
        <div
          class="group my-micro flex items-center justify-start gap-compact border-l-2 border-transparent px-base py-tight transition-all duration-200 hover:bg-surface-hover"
        >
          <p
            class="flex-1 text-xs text-content-muted transition-all duration-200 group-hover:text-content-default"
          >
            Exposed Services
          </p>
          <div
            class="-my-micro flex size-5 items-center justify-center rounded-md border border-transparent bg-surface-hover transition-colors duration-200 group-hover:bg-surface-card hover:border-border-strong"
          >
            <TIcon
              class="size-6 transition-all duration-250 hover:text-content-default"
              :class="{ 'rotate-180': !open }"
              icon="drop-up"
            />
          </div>
        </div>
      </DisclosureButton>
      <DisclosurePanel>
        <template v-if="exposedServices.length > 0">
          <TMenuItem
            v-for="service in filteredExposedServices"
            :key="service.metadata.id"
            :route="service.spec.url"
            :name="service.spec.label!"
            :icon-svg-base64="service.spec.icon_base64"
            icon="window"
            regular-link
          />
          <div
            v-if="errors.length"
            class="flex items-center gap-compact border-l-2 border-transparent px-base text-xs"
          >
            <TIcon icon="exclamation-triangle" class="size-4 text-status-warning-default" />
            <div class="flex-1 truncate text-status-warning-text">
              {{ pluralize('service', errors.length, true) }}
              {{ errors.length === 1 ? 'has' : 'have' }} errors
            </div>
            <Tooltip>
              <template #description>
                <div class="flex flex-col">
                  <div v-for="(error, index) in errors" :key="index">
                    {{ error }}
                  </div>
                </div>
              </template>
              <IconButton icon="question-mark-circle" />
            </Tooltip>
          </div>
        </template>
        <template v-else>
          <p
            class="my-micro items-center justify-start px-base py-tight text-xs text-content-muted"
          >
            No exposed services
          </p>
        </template>
      </DisclosurePanel>
    </template>
  </Disclosure>
</template>
