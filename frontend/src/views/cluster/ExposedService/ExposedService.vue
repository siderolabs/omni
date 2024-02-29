<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <iframe
    :src="exposedServiceUrl"
    class="w-full h-full"
  />
</template>

<script setup lang="ts">
import { RouteLocationRaw, useRoute, useRouter } from "vue-router";
import { ResourceService, ResourceTyped } from "@/api/grpc";
import { ExposedServiceSpec } from "@/api/omni/specs/omni.pb";
import {
  DefaultNamespace,
  ExposedServiceType,
  LabelCluster,
  LabelExposedServiceAlias,
  workloadProxyHostPrefix
} from "@/api/resources";
import { computed, onMounted, ref, Ref, watch } from "vue";
import { withRuntime, withSelectors } from "@/api/options";
import { Runtime } from "@/api/common/omni.pb";
import { checkAuthorized } from "@/router";

const router = useRouter();
const route = useRoute();

const exposedService: Ref<ResourceTyped<ExposedServiceSpec> | undefined> = ref();

const exposedServiceUrl = computed(() => {
  if (!exposedService.value) {
    return "";
  }

  const alias = exposedService.value?.metadata.labels?.[LabelExposedServiceAlias];

  return `${window.location.protocol}//${workloadProxyHostPrefix}-${alias}-${window.location.hostname}`
})

const reloadExposedService = async () => {
  const destRoute = await checkAuthorized(route, true);
  if (destRoute !== true) {
    await router.replace(destRoute as RouteLocationRaw)
  }

  const list = await ResourceService.List({
    type: ExposedServiceType,
    namespace: DefaultNamespace,
  }, withSelectors([
    `${LabelCluster}=${route.params.cluster}`,
    `${LabelExposedServiceAlias}=${route.params.service}`
  ]), withRuntime(Runtime.Omni));

  exposedService.value = list.length > 0 ? list[0] : undefined;
}

watch([() => route.params.cluster, () => route.params.service], reloadExposedService);

onMounted(reloadExposedService)
</script>
