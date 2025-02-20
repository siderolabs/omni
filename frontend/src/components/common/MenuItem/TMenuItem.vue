<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <component :is="componentType" v-bind="componentAttributes" @click.prevent="handleClick">
    <Tooltip placement="right" :description="tooltip" :offset-distance="10" :offset-skid="0">
      <div class="flex flex-col w-full">
        <div class="item w-full" :class="{'sub-item': level > 0, root: level === 0}" :style="{'padding-left': `${24*(level+1)}px`}">
          <t-icon v-if="icon || iconSvgBase64" class="item-icon" :icon="icon" :svg-base64="iconSvgBase64"/>
          <span class="item-name" :id="`sidebar-menu-${name.toLowerCase()}`">{{ name }}</span>
          <div v-if="label" class="item-label" :class="labelColor ? 'text-' + labelColor : ''">
            <span>{{ label }}</span>
          </div>

          <div class="expand-button" v-if="subItems?.length">
            <t-icon
                @click.stop.prevent="() => expanded = !expanded"
                class="w-6 h-6 hover:text-naturals-N13 transition-color transition-transform duration-250"
                :class="{'rotate-180': !expanded}"
                icon="drop-up"
                />
          </div>
        </div>
        <div @click.stop.prevent v-if="expanded" class="relative overflow-hidden" :class="{'submenu-bg': level === 0}">
          <div class="flex gap-2 relative transition-all duration-200" v-for="item, index in (subItems ?? [])" :key="item.name">
            <div class="mx-5 absolute border-l-2 border-b-2 top-0 border-naturals-N8 h-4 z-20 transition-color duration-200"
              :class="{
                'w-2': index === (subItems?.length || 0) - 1 || item.route === $route.path,
                'border-primary-P2': index <= selectedIndex,
              }"
              :style="linePadding"
              />
            <div class="mx-5 absolute border-l-2 top-4 bottom-0 border-naturals-N8 w-2 z-20 transition-color duration-200"
              :class="{
                  'border-primary-P2': index < selectedIndex,
              }"
              :style="linePadding"
              v-if="index != (subItems?.length ?? 0) - 1"/>
            <TMenuItem
              class="flex-1 w-full"
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

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { computed, toRefs } from "vue";
import Tooltip from "../Tooltip/Tooltip.vue";
import { useRoute } from "vue-router";
import storageRef from "@/methods/storage";

type Props = {
  route?: string | object,
  regularLink?: boolean,
  name: string,
  icon?: IconType,
  iconSvgBase64?: string,
  label?: string | number,
  labelColor?: string,
  level?: number
  tooltip?: string
  subItems?: Props[]
};

const props = withDefaults(defineProps<Props>(),
  {
    level: 0
  }
);

const expanded = storageRef(sessionStorage, `sidebar-expanded-${props.level}-${props.name}`, false);
const vueroute = useRoute();
const {
  subItems,
  level
} = toRefs(props);

const handleClick = () => {
  if (!props.route) {
    expanded.value = !expanded.value;
  }
};

const selectedIndex = computed(() => {
  return subItems.value?.findIndex(item => item.route === vueroute.path) ?? -1;
});

const linePadding = computed(() => {
  if (level.value === 0) {
    return {
      'left': '14px'
    };
  }

  return {
    'left': `${24 * level.value + 11}px`
  };
});

const componentType = props.route ? props.regularLink ? "a" : "router-link" : "div";

const componentAttributes = props.route ? props.regularLink ?
    { href: props.route, target: "_blank" } :
    { to: props.route, activeClass: "item-active" } : { class: 'select-none cursor-pointer' };

componentAttributes.class = (componentAttributes.class ?? "") + " item-container";
</script>

<style scoped>
.item {
  @apply flex border-transparent justify-start items-center transition-all duration-200 hover:bg-naturals-N4 py-1.5 my-0.5;
}

.item.root {
  @apply px-6 border-l-2 gap-4;
}

.item:hover .item-icon {
  @apply text-naturals-N13;
}

.item:hover .item-name {
  @apply text-naturals-N13;
}

.item:hover .item-label {
  @apply bg-naturals-N2;
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
  @apply text-xs text-naturals-N11 transition-colors duration-200 flex-1 truncate;
}

.item.sub-item {
  @apply pr-6 gap-2;
}

.item-label {
  @apply rounded-md text-naturals-N13 bg-naturals-N4 text-xs px-1.5 min-w-5 py-0.5 justify-center -my-2 text-center flex items-center font-bold transition-colors duration-200;
}

.expand-button {
  @apply rounded-md bg-naturals-N4 -my-1 transition-colors duration-200 border border-transparent hover:border-naturals-N7 w-5 h-5 flex items-center justify-center;
}

.item:hover .expand-button {
  @apply bg-naturals-N2;
}

.submenu-bg {
  @apply bg-naturals-N0 border-t border-naturals-N4;
}

.item-container:not(:last-child) .submenu-bg {
  @apply border-b;
}

nav:last-of-type  .submenu-bg {
  @apply border-b;
}
</style>
