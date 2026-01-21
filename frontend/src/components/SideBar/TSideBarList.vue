<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

import TMenuItem from '@/components/common/MenuItem/TMenuItem.vue'

import type { IconType } from '../common/Icon/TIcon.vue'

export type SideBarItem = {
  name: string
  route?: string | RouteLocationRaw
  icon?: IconType
  label?: string | number
  labelColor?: string
  tooltip?: string
  iconSvgBase64?: string
  subItems?: SideBarItem[]
  regularLink?: boolean
}

type Props = {
  items: SideBarItem[]
}

defineProps<Props>()
</script>

<template>
  <nav class="pt-2">
    <div class="flex-col">
      <TMenuItem
        v-for="item of items"
        :key="item.name"
        :route="item.route"
        :regular-link="item.regularLink"
        :name="item.name"
        :icon="item.icon"
        :icon-svg-base64="item.iconSvgBase64"
        :label="item.label"
        :label-color="item.labelColor"
        :tooltip="item.tooltip"
        :sub-items="item.subItems"
      />
    </div>
  </nav>
</template>
