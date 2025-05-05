<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <disclosure as="div" class="border-t border-naturals-N4" :defaultOpen="true">
    <template v-slot="{ open }">
      <disclosure-button as="div" class="disclosure">
        <div class="title">
          <p class="title-name truncate">Exposed Services</p>
          <div class="expand-button">
            <t-icon
                class="w-6 h-6 hover:text-naturals-N13 transition-color transition-transform duration-250"
                :class="{'rotate-180': !open}"
                icon="drop-up"
                />
          </div>
        </div>
      </disclosure-button>
      <disclosure-panel>
        <template v-if="exposedServices.length > 0">
          <t-menu-item
              v-for="service in filteredExposedServices"
              :key="service.metadata.id"
              :route="service.spec.url"
              :name="service.spec.label!"
              :icon-svg-base64="service.spec.icon_base64"
              icon="exposed-service"
              regular-link
          />
          <div class="flex gap-4 items-center pl-6 pr-5 text-xs" v-if="errors.length">
            <t-icon icon="warning" class="ml-0.5 text-yellow-Y1"/>
            <div class="text-yellow-Y1 flex-1 truncate">
              {{ pluralize('service', errors.length, true) }} {{ errors.length === 1 ? 'has' : 'have' }} errors
            </div>
            <tooltip>
              <template #description>
                <div class="flex flex-col">
                  <div v-for="error, index in errors" :key="index">
                    {{ error }}
                  </div>
                </div>
              </template>
              <icon-button icon="question" class="mr-0.5"/>
            </tooltip>
          </div>
        </template>
        <template v-else>
          <p class="text-xs text-naturals-N7 justify-start items-center py-1.5 my-1 px-6">No exposed services</p>
        </template>
      </disclosure-panel>
    </template>
  </disclosure>
</template>

<script setup lang="ts">
import { computed, ref, Ref } from "vue";
import { useRoute } from "vue-router";

import { Resource } from "@/api/grpc";
import Watch from "@/api/watch";
import { ExposedServiceSpec } from "@/api/omni/specs/omni.pb";
import { DefaultNamespace, LabelCluster, ExposedServiceType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import TIcon from "@/components/common/Icon/TIcon.vue";
import { Disclosure, DisclosureButton, DisclosurePanel } from "@headlessui/vue";
import TMenuItem from "@/components/common/MenuItem/TMenuItem.vue";
import pluralize from 'pluralize';
import IconButton from "@/components/common/Button/IconButton.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";

const route = useRoute();

const exposedServices: Ref<Resource<ExposedServiceSpec>[]> = ref([]);
const exposedServicesWatch = new Watch(exposedServices);
exposedServicesWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ExposedServiceType,
  },
  selectors: [`${LabelCluster}=${route.params.cluster}`]
});

const filteredExposedServices = computed(() => {
  return exposedServices.value.filter(item => !item.spec.error)
});

const errors = computed(() => {
  const servicesWithErrors = exposedServices.value.filter(item => item.spec.error);

  return servicesWithErrors.map(item => item.spec.error);
});
</script>

<style scoped>
.title {
  @apply flex gap-4 border-l-2 border-transparent justify-start items-center py-1.5 my-1 px-6 transition-all duration-200 hover:bg-naturals-N4;
}

.title:hover .title-name {
  @apply text-naturals-N12;
}

.title-active .title {
  @apply border-primary-P3;
}

.title-active .title-icon {
  @apply text-naturals-N10;
}

.title-active .title-name {
  @apply text-naturals-N10;
}

.title-icon {
  @apply text-naturals-N10 transition-all duration-200;
  width: 16px;
  height: 16px;
}

.title-name {
  @apply text-xs text-naturals-N10 transition-all duration-200 flex-1;
}

.expand-button {
  @apply rounded-md bg-naturals-N4 -my-1 transition-colors duration-200 border border-transparent hover:border-naturals-N7 w-5 h-5 flex items-center justify-center;
}

.title:hover .expand-button {
  @apply bg-naturals-N2;
}
</style>
