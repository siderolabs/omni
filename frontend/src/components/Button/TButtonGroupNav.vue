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

import TButtonGroupItem from './primitives/TButtonGroupItem.vue'
import TButtonGroupRoot from './primitives/TButtonGroupRoot.vue'

// Looks like TButtonGroup, but each item is a link to a page rather than a value.
const props = defineProps<{
  // Names the navigation for screen readers, which list every <nav> on the page by it.
  label: string
  class?: ClassValue
  items: {
    label: string
    to: RouteLocationRaw
    disabled?: boolean
  }[]
}>()
</script>

<template>
  <!-- Scrolls sideways when the group is wider than the screen -->
  <NavigationMenuRoot :aria-label="label" :class="cn('flex overflow-x-auto', props.class)">
    <TButtonGroupRoot class="shrink-0 whitespace-nowrap">
      <NavigationMenuList>
        <NavigationMenuItem v-for="item in items" :key="item.label" class="flex">
          <RouterLink v-slot="{ href, navigate, isExactActive }" :to="item.to" custom>
            <!-- A disabled item can still be current, when its page is open by URL -->
            <TButtonGroupItem class="px-tight py-micro">
              <span
                v-if="item.disabled"
                role="link"
                aria-disabled="true"
                data-disabled
                :aria-current="isExactActive ? 'page' : undefined"
                :data-state="isExactActive ? 'checked' : 'unchecked'"
              >
                {{ item.label }}
              </span>

              <NavigationMenuLink v-else as-child :active="isExactActive">
                <a :href :data-state="isExactActive ? 'checked' : 'unchecked'" @click="navigate">
                  {{ item.label }}
                </a>
              </NavigationMenuLink>
            </TButtonGroupItem>
          </RouterLink>
        </NavigationMenuItem>
      </NavigationMenuList>
    </TButtonGroupRoot>
  </NavigationMenuRoot>
</template>
