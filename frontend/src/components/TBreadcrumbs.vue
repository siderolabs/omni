<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { type RouteLocationRaw, useRoute } from 'vue-router'

type Props = {
  nodeName?: string
  last?: string
}

const { last } = defineProps<Props>()

const route = useRoute()

const breadcrumbs = computed(() => {
  const res: {
    text: string
    to?: RouteLocationRaw
  }[] = []

  if ('cluster' in route.params) {
    res.push({
      text: route.params.cluster,
      to: {
        name: 'ClusterOverview',
        params: {
          cluster: route.params.cluster,
        },
      },
    })

    if ('machine' in route.params) {
      res.push({
        text: route.params.machine,
        to: {
          name: 'NodeOverview',
          params: {
            cluster: route.params.cluster,
            machine: route.params.machine,
          },
        },
      })
    }
  }

  if (last) {
    res.push({
      text: last,
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
          {{
            'machine' in $route.params && crumb.text === $route.params.machine && !!nodeName
              ? nodeName
              : crumb.text
          }}
        </RouterLink>
        <p
          v-if="idx === breadcrumbs.length - 1"
          class="cursor-default text-xl font-medium break-all text-naturals-n14"
        >
          {{
            'machine' in $route.params && crumb.text === $route.params.machine && !!nodeName
              ? nodeName
              : crumb.text
          }}
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
