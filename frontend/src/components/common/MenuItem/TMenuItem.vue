<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <component :is="componentType" v-bind="componentAttributes">
    <Tooltip placement="right" :description="tooltip" :offset-distance="10" :offset-skid="0">
      <div :class="{'border-naturals-N5': expanded, 'border-transparent': !expanded}" class="border-b border-t divide-naturals-N5 flex flex-col">
        <div class="item" :class="{'sub-item': subItem}">
          <t-icon v-if="icon" class="item-icon" :icon="icon" :svg-base64="iconSvgBase64"/>
          <p class="item-name truncate">{{ name }}</p>
          <div v-if="label" class="rounded-md text-naturals-N13 bg-naturals-N4 text-xs px-1.5 min-w-5 py-0.5 justify-center -my-2 text-center flex items-center font-bold" :class="labelColor ? 'text-' + labelColor : ''">
            <span>{{ label }}</span>
          </div>
          <t-icon v-if="hasSubItems"
            icon="arrow-up"
            class="w-4 h-4 hover:text-naturals-N13 transition-color transition-transform duration-250"
            :class="{'rotate-180': expanded}"
            @click.stop.prevent="() => expanded = !expanded"
            />
        </div>
        <div @click.stop.prevent v-if="expanded">
          <slot/>
        </div>
      </div>
    </Tooltip>
  </component>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { ref } from "vue";
import Tooltip from "../Tooltip/Tooltip.vue";

type Props = {
  route: string | object,
  regularLink?: boolean,
  name: string,
  icon?: IconType,
  iconSvgBase64?: string,
  label?: string | number,
  labelColor?: string,
  hasSubItems?: boolean
  subItem?: boolean
  tooltip?: string
};

const props = defineProps<Props>();
const expanded = ref(false);

const componentType = props.regularLink ? "a" : "router-link";
const componentAttributes = props.regularLink ?
    { href: props.route, target: "_blank" } :
    { to: props.route, activeClass: "item-active" };
</script>

<style scoped>
.item {
  @apply flex gap-4 border-l-2 border-transparent justify-start items-center py-1.5 my-0.5 px-6 transition-all duration-200 hover:bg-naturals-N4;
}
.item:hover .item-icon {
  @apply text-naturals-N13;
}
.item:hover .item-name {
  @apply text-naturals-N13;
}
.item-active .item {
  @apply border-primary-P3;
}
.item-active .item-icon {
  @apply text-naturals-N13;
}
.item-active .item-name {
  @apply text-naturals-N13;
}
.item-icon {
  @apply text-naturals-N11 transition-all duration-200;
  width: 16px;
  height: 16px;
}
.item-name {
  @apply text-xs text-naturals-N11 transition-all duration-200 flex-1;
}

.item.sub-item {
  @apply pl-12;
}
</style>
