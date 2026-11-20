<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { RouteNamedMap } from 'vue-router/auto-routes'

/** The routes of a machine's pages, in a cluster (Node*) or outside one (Machine*). */
export type MachinePageRoute = Extract<keyof RouteNamedMap, `Node${string}` | `Machine${string}`>

export interface MachinePageTab {
  name: string
  /** Shown beside the name, e.g. how many containers the tab lists. */
  count?: number
  /** A tab with one page opens it; a tab with several lists them in a second row. */
  pages: {
    name: string
    route: MachinePageRoute
    disabled?: boolean
    /** Routes without a tab of their own that belong to this page, e.g. editing a patch. */
    alsoOn?: MachinePageRoute[]
  }[]
}
</script>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import TButtonGroupNav from '@/components/Button/TButtonGroupNav.vue'
import TabNav from '@/components/Tabs/TabNav.vue'

// The tabs of a machine's page, shared by the page in a cluster and the one outside it so
// the same pages sit under the same tabs in both. Each page keeps its own route.
const { tabs } = defineProps<{ tabs: MachinePageTab[] }>()

const route = useRoute()

// A location per route name. TypeScript won't split { name: A | B } into { name: A } | { name: B }
// by itself, and RouteLocationRaw only accepts the split form.
type MachinePageLocation = MachinePageRoute extends infer R
  ? R extends MachinePageRoute
    ? { name: R }
    : never
  : never

const to = (name: MachinePageRoute) => ({ name }) as MachinePageLocation

const isOn = (page: MachinePageTab['pages'][number]) =>
  page.route === route.name || !!page.alsoOn?.some((r) => r === route.name)

// A tab is current while any of its pages is
const activeTab = computed(() => tabs.find((t) => t.pages.some(isOn)))

const tabItems = computed(() =>
  tabs.map((tab) => {
    // A tab opens the first of its pages the user is allowed to read
    const page = tab.pages.find((p) => !p.disabled)

    return {
      label: tab.name,
      count: tab.count,
      to: to(page?.route ?? tab.pages[0].route),
      disabled: !page,
      current: tab === activeTab.value,
    }
  }),
)

// The second row, for a tab that groups several pages
const subNav = computed(() => {
  if (!activeTab.value || activeTab.value.pages.length < 2) return

  return {
    label: activeTab.value.name,
    items: activeTab.value.pages.map((p) => ({
      label: p.name,
      to: to(p.route),
      disabled: p.disabled,
    })),
  }
})
</script>

<template>
  <div class="flex shrink-0 flex-col">
    <TabNav label="Machine" class="px-compact md:px-base" :items="tabItems" />

    <TButtonGroupNav
      v-if="subNav"
      v-bind="subNav"
      class="mx-compact mt-tight md:mx-base md:mt-compact"
    />
  </div>
</template>
