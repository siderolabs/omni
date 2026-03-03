<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { type AnchorHTMLAttributes, type ButtonHTMLAttributes, computed } from 'vue'
import { type RouterLinkProps, useRoute } from 'vue-router'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

import Tooltip from '../Tooltip/Tooltip.vue'

type Props = {
  route?: string | object
  regularLink?: boolean
  name: string
  icon?: IconType
  iconSvgBase64?: string
  label?: string | number
  labelDanger?: boolean
  level?: number
  tooltip?: string
  subItems?: Props[]
}

const { name, level = 0, subItems, route, regularLink } = defineProps<Props>()

const expanded = useSessionStorage(() => `sidebar-expanded-${level}-${name}`, false)
const vueroute = useRoute()

/**
 * The force logic here is to cater for routes with children.
 * For routes with children, you can only toggle the submenu
 * by clicking the arrow icon.
 *
 * If the parent is not itself a route, clicking the title
 * is enough.
 */
const toggleSubmenu = (force = false) => {
  if (!force && route) return

  if (subItems?.length) {
    expanded.value = !expanded.value
  }
}

const selectedIndex = computed(() => {
  return subItems?.findIndex((item) => item.route === vueroute.path) ?? -1
})

const linePadding = computed(() => {
  if (level === 0) {
    return {
      left: '14px',
    }
  }

  return {
    left: `${24 * level + 11}px`,
  }
})

const componentType = computed(() => {
  if (!route) return 'div'

  return regularLink ? 'a' : 'router-link'
})

const componentAttributes = computed(() => {
  if (!route) {
    return {
      role: 'button',
    } as ButtonHTMLAttributes
  }

  if (regularLink) {
    return {
      href: route,
      target: '_blank',
      rel: 'noopener noreferrer',
    } as AnchorHTMLAttributes
  }

  return {
    to: route,
  } as RouterLinkProps
})
</script>

<template>
  <component :is="componentType" class="group/tree select-none" v-bind="componentAttributes">
    <Tooltip placement="right" :description="tooltip" :offset-distance="10" :offset-skid="0">
      <div class="flex w-full flex-col">
        <div
          class="group/item my-0.5 flex w-full items-center justify-start border-transparent py-1.5 transition-all duration-200 group-aria-[current]/tree:border-primary-p3 hover:bg-naturals-n4"
          :class="{ 'gap-2 pr-6': level > 0, 'gap-4 border-l-2 px-6': level === 0 }"
          :style="{ 'padding-left': `${1.5 * (level + 1)}rem` }"
          @click="() => toggleSubmenu()"
        >
          <TIcon
            v-if="icon || iconSvgBase64"
            class="size-4 text-naturals-n11 transition-all duration-200 group-hover/item:text-naturals-n13 group-aria-[current]/tree:text-naturals-n13"
            :icon="icon"
            :svg-base64="iconSvgBase64"
          />
          <span
            :id="`sidebar-menu-${name.toLowerCase()}`"
            class="flex-1 truncate text-xs text-naturals-n11 transition-colors duration-200 group-hover/item:text-naturals-n13 group-aria-[current]/tree:text-naturals-n13"
          >
            {{ name }}
          </span>
          <div
            v-if="label"
            class="-my-2 flex min-w-5 items-center justify-center rounded-md bg-naturals-n4 px-1.5 py-0.5 text-center text-xs font-bold transition-colors duration-200 group-hover/item:bg-naturals-n2"
            :class="labelDanger ? 'text-red-r1' : 'text-naturals-n13'"
          >
            <span>{{ label }}</span>
          </div>

          <div
            v-if="subItems?.length"
            class="-my-1 flex h-5 w-5 items-center justify-center rounded-md border border-transparent bg-naturals-n4 transition-colors duration-200 group-hover/item:border-naturals-n7 group-hover/item:bg-naturals-n2"
            role="button"
            @click.stop.prevent="() => toggleSubmenu(true)"
          >
            <TIcon
              class="transition-color h-6 w-6 transition-transform duration-250 group-hover/item:text-naturals-n13"
              :class="{ 'rotate-180': !expanded }"
              icon="drop-up"
            />
          </div>
        </div>

        <div
          v-if="expanded"
          class="relative overflow-hidden"
          :class="{ 'border-y border-naturals-n4 bg-naturals-n0': level === 0 }"
        >
          <div
            v-for="(item, index) in subItems ?? []"
            :key="item.name"
            class="relative flex gap-2 transition-all duration-200"
          >
            <div
              class="transition-color absolute top-0 z-20 mx-5 h-4 border-b-2 border-l-2 duration-200"
              :class="[
                index <= selectedIndex ? 'border-primary-p2' : 'border-naturals-n8',
                { 'w-2': index === (subItems?.length || 0) - 1 || item.route === $route.path },
              ]"
              :style="linePadding"
            />
            <div
              v-if="index !== (subItems?.length ?? 0) - 1"
              class="transition-color absolute top-4 bottom-0 z-20 mx-5 w-2 border-l-2 duration-200"
              :class="index < selectedIndex ? 'border-primary-p2' : 'border-naturals-n8'"
              :style="linePadding"
            />
            <TMenuItem
              class="w-full flex-1"
              :route="item.route"
              :regular-link="item.regularLink"
              :name="item.name"
              :icon="item.icon"
              :icon-svg-base64="item.iconSvgBase64"
              :label="item.label"
              :label-danger="item.labelDanger"
              :level="level + 1"
              :sub-item="true"
              :tooltip="item.tooltip"
              :sub-items="item.subItems"
            />
          </div>
        </div>
      </div>
    </Tooltip>
  </component>
</template>
