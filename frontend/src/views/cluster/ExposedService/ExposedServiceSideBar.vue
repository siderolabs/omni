<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <disclosure as="div" class="border-t border-naturals-N4" :defaultOpen="true">
    <template v-slot="{ open }">
      <disclosure-button as="div" class="disclosure">
        <div class="title">
          <p class="title__name truncate">Exposed Services</p>
          <t-icon icon="arrow-up" class="title__icon" :class="{'rotate-180': open}"/>
        </div>
      </disclosure-button>
      <disclosure-panel>
        <template v-if="exposedServices.length > 0">
          <t-menu-item
              v-for="service in exposedServices"
              :key="service.metadata.id"
              :route="exposedServiceUrl(service)"
              :name="service.spec.label"
              :icon-svg-base64="service.spec.icon_base64"
              icon="exposed-service"
              regular-link
          />
        </template>
        <template v-else>
          <p class="text-xs text-naturals-N7 justify-start items-center py-1.5 my-1 px-6">No exposed services</p>
        </template>
      </disclosure-panel>
    </template>
  </disclosure>
</template>

<script setup lang="ts">
import { ref, Ref } from "vue";
import { useRoute } from "vue-router";

import { ResourceTyped } from "@/api/grpc";
import Watch from "@/api/watch";
import { ExposedServiceSpec } from "@/api/omni/specs/omni.pb";
import { DefaultNamespace, LabelCluster, ExposedServiceType, LabelExposedServiceAlias, workloadProxyHostPrefix } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import TIcon from "@/components/common/Icon/TIcon.vue";
import { Disclosure, DisclosureButton, DisclosurePanel } from "@headlessui/vue";
import TMenuItem from "@/components/common/MenuItem/TMenuItem.vue";

const route = useRoute();

const exposedServices: Ref<ResourceTyped<ExposedServiceSpec>[]> = ref([]);
const exposedServicesWatch = new Watch(exposedServices);
exposedServicesWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ExposedServiceType,
  },
  selectors: [`${LabelCluster}=${route.params.cluster}`]
});

const exposedServiceUrl = (service: ResourceTyped<ExposedServiceSpec>) => {
  const alias = service.metadata.labels?.[LabelExposedServiceAlias];

  return `${window.location.protocol}//${workloadProxyHostPrefix}-${alias}-${window.location.hostname}`
}
</script>

<style scoped>
.title {
  @apply flex gap-4 border-l-2 border-transparent justify-start items-center py-1.5 my-1 px-6 transition-all duration-200 hover:bg-naturals-N4;
}

.title:hover .title__icon {
  @apply text-naturals-N12;
}

.title:hover .title__name {
  @apply text-naturals-N12;
}

.title__active .title {
  @apply border-primary-P3;
}

.title__active .title__icon {
  @apply text-naturals-N10;
}

.title__active .title__name {
  @apply text-naturals-N10;
}

.title__icon {
  @apply text-naturals-N10 transition-all duration-200;
  width: 16px;
  height: 16px;
}

.title__name {
  @apply text-xs text-naturals-N10 transition-all duration-200 flex-1;
}
</style>
