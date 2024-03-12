<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <component :is="componentType" v-bind="componentAttributes">
    <div class="item">
      <t-icon v-if="icon" class="item__icon" :icon="icon" :svg-base64="iconSvgBase64"/>
      <p class="item__name truncate">{{ name }}</p>
      <div v-if="label" class="rounded-full text-naturals-N13 bg-naturals-N4 text-xs p-1 w-6 h-6 text-center" :class="labelColor ? 'text-' + labelColor : ''">
        {{ label }}
      </div>
    </div>
  </component>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";

type Props = {
  route: string | object,
  regularLink?: boolean,
  name: string,
  icon?: IconType,
  iconSvgBase64?: string,
  label?: string | number,
  labelColor?: string
};

const props = defineProps<Props>();

const componentType = props.regularLink ? "a" : "router-link";
const componentAttributes = props.regularLink ?
    { href: props.route, target: "_blank" } :
    { to: props.route, activeClass: "item__active" };
</script>

<style scoped>
.item {
  @apply flex gap-4 border-l-2 border-transparent justify-start items-center py-1.5 my-1 px-6 transition-all duration-200 hover:bg-naturals-N4;
}
.item:hover .item__icon {
  @apply text-naturals-N13;
}
.item:hover .item__name {
  @apply text-naturals-N13;
}
.item__active .item {
  @apply border-primary-P3;
}
.item__active .item__icon {
  @apply text-naturals-N13;
}
.item__active .item__name {
  @apply text-naturals-N13;
}
.item__icon {
  @apply text-naturals-N11 transition-all duration-200;
  width: 16px;
  height: 16px;
}
.item__name {
  @apply text-xs text-naturals-N11 transition-all duration-200 flex-1;
}
</style>
