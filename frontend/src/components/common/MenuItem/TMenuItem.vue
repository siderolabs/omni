<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { computed, toRefs } from 'vue'
import { useRoute } from 'vue-router'

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
  labelColor?: string
  level?: number
  tooltip?: string
  subItems?: Props[]
}

const props = withDefaults(defineProps<Props>(), {
  level: 0,
})

const expanded = useSessionStorage(`sidebar-expanded-${props.level}-${props.name}`, false)
const vueroute = useRoute()
const { subItems, level } = toRefs(props)

/**
 * The force logic here is to cater for routes with children.
 * For routes with children, you can only toggle the submenu
 * by clicking the arrow icon.
 *
 * If the parent is not itself a route, clicking the title
 * is enough.
 */
const toggleSubmenu = (force = false) => {
  if (!force && props.route) return

  if (subItems.value?.length) {
    expanded.value = !expanded.value
  }
}

const selectedIndex = computed(() => {
  return subItems.value?.findIndex((item) => item.route === vueroute.path) ?? -1
})

const linePadding = computed(() => {
  if (level.value === 0) {
    return {
      left: '14px',
    }
  }

  return {
    left: `${24 * level.value + 11}px`,
  }
})

const componentType = props.route ? (props.regularLink ? 'a' : 'router-link') : 'div'

const componentAttributes = props.route
  ? props.regularLink
    ? { href: props.route, target: '_blank', rel: 'noopener noreferrer' }
    : { to: props.route, exactActiveClass: 'item-active' }
  : { class: 'select-none cursor-pointer', role: 'button' }

componentAttributes.class = (componentAttributes.class ?? '') + ' item-container'
</script>

<template>
  <component :is="componentType" v-bind="componentAttributes">
    <Tooltip placement="right" :description="tooltip" :offset-distance="10" :offset-skid="0">
      <div class="flex w-full flex-col">
        <div
          class="item w-full"
          :class="{ 'sub-item': level > 0, root: level === 0 }"
          :style="{ 'padding-left': `${24 * (level + 1)}px` }"
          @click="() => toggleSubmenu()"
        >
          <TIcon
            v-if="icon || iconSvgBase64"
            class="item-icon"
            :icon="icon"
            :svg-base64="iconSvgBase64"
          />
          <span :id="`sidebar-menu-${name.toLowerCase()}`" class="item-name">{{ name }}</span>
          <div v-if="label" class="item-label" :class="labelColor ? 'text-' + labelColor : ''">
            <span>{{ label }}</span>
          </div>

          <div
            v-if="subItems?.length"
            class="expand-button"
            role="button"
            @click.stop.prevent="() => toggleSubmenu(true)"
          >
            <TIcon
              class="transition-color h-6 w-6 transition-transform duration-250 hover:text-naturals-n13"
              :class="{ 'rotate-180': !expanded }"
              icon="drop-up"
            />
          </div>
        </div>
        <div
          v-if="expanded"
          class="relative overflow-hidden"
          :class="{ 'submenu-bg': level === 0 }"
        >
          <div
            v-for="(item, index) in subItems ?? []"
            :key="item.name"
            class="relative flex gap-2 transition-all duration-200"
          >
            <div
              class="transition-color absolute top-0 z-20 mx-5 h-4 border-b-2 border-l-2 border-naturals-n8 duration-200"
              :class="{
                'w-2': index === (subItems?.length || 0) - 1 || item.route === $route.path,
                'border-primary-p2': index <= selectedIndex,
              }"
              :style="linePadding"
            />
            <div
              v-if="index !== (subItems?.length ?? 0) - 1"
              class="transition-color absolute top-4 bottom-0 z-20 mx-5 w-2 border-l-2 border-naturals-n8 duration-200"
              :class="{
                'border-primary-p2': index < selectedIndex,
              }"
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
              :label-color="item.labelColor"
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

<style scoped>
@reference "../../../index.css";

.item {
  @apply my-0.5 flex items-center justify-start border-transparent py-1.5 transition-all duration-200 hover:bg-naturals-n4;
}

.item.root {
  @apply gap-4 border-l-2 px-6;
}

.item:hover .item-icon {
  @apply text-naturals-n13;
}

.item:hover .item-name {
  @apply text-naturals-n13;
}

.item:hover .item-label {
  @apply bg-naturals-n2;
}

.item-active .item {
  @apply border-primary-p3;
}

.item-active .item-icon {
  @apply text-naturals-n13;
}

.item-active .item-name {
  @apply text-naturals-n13;
}

.item-icon {
  @apply text-naturals-n11 transition-all duration-200;
  width: 16px;
  height: 16px;
}

.item-name {
  @apply flex-1 truncate text-xs text-naturals-n11 transition-colors duration-200;
}

.item.sub-item {
  @apply gap-2 pr-6;
}

.item-label {
  @apply -my-2 flex min-w-5 items-center justify-center rounded-md bg-naturals-n4 px-1.5 py-0.5 text-center text-xs font-bold text-naturals-n13 transition-colors duration-200;
}

.expand-button {
  @apply -my-1 flex h-5 w-5 items-center justify-center rounded-md border border-transparent bg-naturals-n4 transition-colors duration-200 hover:border-naturals-n7;
}

.item:hover .expand-button {
  @apply bg-naturals-n2;
}

.submenu-bg {
  @apply border-t border-naturals-n4 bg-naturals-n0;
}

.item-container:not(:last-child) .submenu-bg {
  @apply border-b;
}

nav:last-of-type .submenu-bg {
  @apply border-b;
}
</style>
