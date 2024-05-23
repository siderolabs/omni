<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="label">
    <t-icon
      class="label-icon"
      :style="{
        fill: !!color ? color : iconData?.iconColor || iconColor.color,
      }"
      :icon="!!iconType ? iconType : iconData?.iconTypeValue"
    />
    <span
      class="label-title"
      :style="{
        color: !!color ? color : iconData?.iconColor || iconColor.color,
      }"
      v-if="title"
      >{{ title }}</span
    >
  </div>
</template>

<script setup lang="ts">
import { computed, toRefs } from "vue";
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import {
  NodesViewFilterOptions,
  TPodsViewFilterOptions,
  TCommonStatuses,
} from "@/constants";

type Props = {
  iconType?: IconType;
  title?: string;
  color?: string;
};

const props = defineProps<Props>();
const { iconType, title, color } = toRefs(props);

const iconData = computed((): { iconColor?: string, iconTypeValue?: IconType } => {
  if (title.value) {
    switch (title.value) {
      case TPodsViewFilterOptions.RUNNING:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case NodesViewFilterOptions.READY:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TPodsViewFilterOptions.PENDING:
        return {
          iconTypeValue: "time",
          iconColor: "#FFB200",
        };
      case TPodsViewFilterOptions.FAILED:
        return {
          iconTypeValue: "error",
          iconColor: "#FF5C56",
        };
      case TPodsViewFilterOptions.UNKNOWN:
        return {
          iconTypeValue: "unknown",
          iconColor: "#FF8B59",
        }
      case TPodsViewFilterOptions.SUCCEEDED:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TCommonStatuses.COMPLETED:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TCommonStatuses.FAILED:
        return {
          iconTypeValue: "error",
          iconColor: "#FF5C56",
        };
      case TCommonStatuses.ERROR:
        return {
          iconTypeValue: "error",
          iconColor: "#FF5C56",
        };
      case NodesViewFilterOptions.NOT_READY:
        return {
          iconTypeValue: "time",
          iconColor: "#FF5C56",
        };
      case TCommonStatuses.WAITING:
      case TCommonStatuses.LOADING:
        return {
          iconTypeValue: "time",
          iconColor: "#FF8B59",
        };
      case TCommonStatuses.UNKNOWN:
        return {
          iconTypeValue: "unknown",
          iconColor: "#FF8B59",
        };
      case TCommonStatuses.HEALTH_UNKNOWN:
        return {
          iconTypeValue: "question",
          iconColor: "#7D7D85",
        };
      case TCommonStatuses.HEALTHY:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TCommonStatuses.UNHEALTHY:
        return {
          iconTypeValue: "error",
          iconColor: "#FF5C56",
        };
      case TCommonStatuses.ON:
        return {
          iconTypeValue: "dot",
          iconColor: "#69C297",
        };
      case TCommonStatuses.OFF:
        return {
          iconTypeValue: "dot",
          iconColor: "#7D7D85",
        };
      case TCommonStatuses.TRUE:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#7D7D85",
        };
      case TCommonStatuses.FALSE:
        return {
          iconTypeValue: "error",
          iconColor: "#7D7D85",
        };
      case TCommonStatuses.UP_TO_DATE:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TCommonStatuses.OUTDATED:
        return {
          iconTypeValue: "error",
          iconColor: "#FF5C56",
        };
      case TCommonStatuses.APPLIED:
        return {
          iconTypeValue: "check-in-circle-classic",
          iconColor: "#69C297",
        };
      case TCommonStatuses.PENDING:
        return {
          iconTypeValue: "time",
          iconColor: "#FFB200",
        };
      default:
        return {
          iconTypeValue: "unknown",
          iconColor: "#FF8B59",
        }
    }
  }

  return {};
});

const iconColor = computed(() => {
  if (iconType.value) {
    switch (iconType.value) {
      case "check-in-circle-classic":
        return { color: "#69C297" };
      case "loading":
        return { color: "#FFB200" };
      case "time":
        return { color: "#7D7D85" };
      case "refresh":
        return { color: "#59A5FF" };
      default:
        return { color: "#7D7D85" };
    }
  }

  return {};
})
</script>

<style scoped>
.label {
  @apply flex items-center;
}
.label-icon {
  @apply fill-current;
  width: 16px;
  height: 16px;
}
.label-title {
  @apply pl-1 text-xs;
}
</style>
