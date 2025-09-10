<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
import { type LocationQuery, useRoute } from 'vue-router'

type Props = {
  nodeName?: string
  last?: string
}
const props = defineProps<Props>()

const { last } = toRefs(props)

const route = useRoute()

const breadcrumbs = computed(() => {
  const res: { text: string; to?: { name: string; query: LocationQuery } }[] = []

  if (route.params.cluster) {
    res.push(
      {
        text: `${route.params.cluster}`,
        to: { name: 'ClusterOverview', query: route.query },
      },
      {
        text: `${route.params.machine}`,
        to: { name: 'NodeOverview', query: route.query },
      },
    )
  }

  if (last?.value) {
    res.push({
      text: last.value,
    })
  }

  return res
})
</script>

<template>
  <div v-if="breadcrumbs.length > 0" class="items-startflex-row flex flex-wrap">
    <template v-for="(crumb, idx) in breadcrumbs" :key="crumb.text">
      <div class="flex items-center">
        <RouterLink
          v-if="idx !== breadcrumbs.length - 1"
          class="text-xl font-medium transition hover:opacity-50"
          :to="crumb.to!"
        >
          {{ crumb.text === $route.params.machine && !!nodeName ? nodeName : crumb.text }}
        </RouterLink>
        <p
          v-if="idx === breadcrumbs.length - 1"
          class="cursor-default text-xl font-medium break-all text-naturals-n14"
        >
          {{ crumb.text === $route.params.machine && !!nodeName ? nodeName : crumb.text }}
        </p>
        <svg
          v-if="idx < breadcrumbs.length - 1"
          class="h-5 w-5 shrink-0 opacity-50"
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
