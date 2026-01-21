<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import pluralize from 'pluralize'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { ExposedServiceSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, ExposedServiceType, LabelCluster } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TMenuItem from '@/components/common/MenuItem/TMenuItem.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const route = useRoute()

const { data: exposedServices } = useResourceWatch<ExposedServiceSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ExposedServiceType,
  },
  selectors: [`${LabelCluster}=${route.params.cluster}`],
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
  <Disclosure as="div" class="border-t border-naturals-n4" default-open>
    <template #default="{ open }">
      <DisclosureButton as="div" class="disclosure">
        <div class="title">
          <p class="title-name truncate">Exposed Services</p>
          <div class="expand-button">
            <TIcon
              class="transition-color h-6 w-6 transition-transform duration-250 hover:text-naturals-n13"
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
            icon="exposed-service"
            regular-link
          />
          <div v-if="errors.length" class="flex items-center gap-4 pr-5 pl-6 text-xs">
            <TIcon icon="warning" class="ml-0.5 text-yellow-y1" />
            <div class="flex-1 truncate text-yellow-y1">
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
              <IconButton icon="question" class="mr-0.5" />
            </Tooltip>
          </div>
        </template>
        <template v-else>
          <p class="my-1 items-center justify-start px-6 py-1.5 text-xs text-naturals-n7">
            No exposed services
          </p>
        </template>
      </DisclosurePanel>
    </template>
  </Disclosure>
</template>

<style scoped>
@reference "../../../index.css";

.title {
  @apply my-1 flex items-center justify-start gap-4 border-l-2 border-transparent px-6 py-1.5 transition-all duration-200 hover:bg-naturals-n4;
}

.title:hover .title-name {
  @apply text-naturals-n12;
}

.title-active .title {
  @apply border-primary-p3;
}

.title-active .title-icon {
  @apply text-naturals-n10;
}

.title-active .title-name {
  @apply text-naturals-n10;
}

.title-icon {
  @apply text-naturals-n10 transition-all duration-200;
  width: 16px;
  height: 16px;
}

.title-name {
  @apply flex-1 text-xs text-naturals-n10 transition-all duration-200;
}

.expand-button {
  @apply -my-1 flex h-5 w-5 items-center justify-center rounded-md border border-transparent bg-naturals-n4 transition-colors duration-200 hover:border-naturals-n7;
}

.title:hover .expand-button {
  @apply bg-naturals-n2;
}
</style>
