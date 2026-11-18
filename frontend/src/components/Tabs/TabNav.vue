<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
  NavigationMenuRoot,
} from 'reka-ui'
import type { ClassValue } from 'vue'
import { type RouteLocationRaw, RouterLink } from 'vue-router'

import { cn } from '@/methods/utils'

import TabItem from './primitives/TabItem.vue'

// Looks like Tabs, but each tab is a link to a page rather than a panel on this one.
const props = defineProps<{
  // Names the navigation for screen readers, which list every <nav> on the page by it.
  label: string
  class?: ClassValue
  items: {
    label: string
    to: RouteLocationRaw
    disabled?: boolean
    // Marks a tab as current when the page is one of several it stands for, not just
    // the one it links to. Defaults to whether the link's own route is the current one.
    current?: boolean
  }[]
}>()

// Tabs draws its underline with one sliding TabsIndicator; here each link draws its own,
// over the divider at the bottom of the list. The list's padding leaves room for the
// links' focus ring, which the scrolling list would otherwise clip.
const linkClass =
  'relative flex data-[state=active]:after:absolute data-[state=active]:after:inset-x-0 data-[state=active]:after:-bottom-snug data-[state=active]:after:h-0.5 data-[state=active]:after:bg-accent-default'
</script>

<template>
  <NavigationMenuRoot :aria-label="label" class="shrink-0">
    <NavigationMenuList
      :class="
        cn(
          'flex gap-base overflow-x-auto pt-micro pb-snug whitespace-nowrap shadow-[inset_0_-1px_0_var(--color-border-default)]',
          props.class,
        )
      "
    >
      <NavigationMenuItem v-for="item in items" :key="item.label" class="flex">
        <RouterLink v-slot="{ href, navigate, isExactActive }" :to="item.to" custom>
          <!--
            Our own elements under as-child, so our aria-current wins over reka's: "page" only
            on the page itself, "true" on a tab standing for it. A disabled tab can still be
            current, when its page is open by URL.
          -->
          <TabItem :class="linkClass">
            <span
              v-if="item.disabled"
              role="link"
              aria-disabled="true"
              data-disabled
              :aria-current="isExactActive ? 'page' : item.current ? 'true' : undefined"
              :data-state="(item.current ?? isExactActive) ? 'active' : 'inactive'"
            >
              {{ item.label }}
            </span>

            <NavigationMenuLink v-else as-child :active="isExactActive">
              <a
                :href
                :aria-current="isExactActive ? 'page' : item.current ? 'true' : undefined"
                :data-state="(item.current ?? isExactActive) ? 'active' : 'inactive'"
                @click="navigate"
              >
                {{ item.label }}
              </a>
            </NavigationMenuLink>
          </TabItem>
        </RouterLink>
      </NavigationMenuItem>
    </NavigationMenuList>
  </NavigationMenuRoot>
</template>
