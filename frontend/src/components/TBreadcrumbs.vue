<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-startflex-row flex-wrap" v-if="breadcrumbs.length > 0">
    <template v-for="(crumb, idx) in breadcrumbs" :key="crumb.text">
      <div class="flex items-center">
        <router-link
          v-if="idx !== breadcrumbs.length - 1"
          class="all transition"
          :to="crumb.to"
          >{{
            crumb.text === $route.params.machine && !!nodeName
              ? nodeName
              : crumb.text
          }}</router-link
        >
        <p v-if="idx == breadcrumbs.length - 1" class="last">
          {{
            crumb.text === $route.params.machine && !!nodeName
              ? nodeName
              : crumb.text
          }}
        </p>
        <svg
          v-if="idx < breadcrumbs.length - 1"
          class="flex-shrink-0 h-5 w-5 text-opacity-50"
          xmlns="http://www.w3.org/2000/svg"
          fill="currentColor"
          viewBox="0 0 20 20"
          aria-hidden="true"
        >
          <path d="M5.555 17.776l8-16 .894.448-8 16-.894-.448z" />
        </svg>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";
import { computed, toRefs } from "vue";
import { getBreadcrumbs } from "@/router";

type Props = {
  nodeName?: string,
  last?: string
};
const props = defineProps<Props>();

const { last } = toRefs(props);

const route = useRoute();

const breadcrumbs = computed(() => {
  const res = getBreadcrumbs(route);

  if (last?.value) {
    res.push({
      text: last.value
    })
  }

  return res;
});
</script>

<style scoped>
.all {
  @apply text-xl font-medium hover:text-opacity-50;
}
.last {
  @apply text-xl font-medium text-naturals-N14 break-all cursor-default;
}
.nodeName__multiple {
  @apply flex items-center;
}
</style>
