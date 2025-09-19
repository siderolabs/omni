<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { NodesViewFilterOptions, TCommonStatuses, TPodsViewFilterOptions } from '@/constants'

type Props = {
  iconType?: IconType
  title?: string
  color?: string
}

const props = defineProps<Props>()
const { iconType, title, color } = toRefs(props)

const iconData = computed((): { iconColor?: string; iconTypeValue?: IconType } => {
  if (title.value) {
    switch (title.value) {
      case TPodsViewFilterOptions.RUNNING:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color-green-g1)',
        }
      case NodesViewFilterOptions.READY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color-green-g1)',
        }
      case TPodsViewFilterOptions.PENDING:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color-yellow-y1)',
        }
      case TPodsViewFilterOptions.FAILED:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case TPodsViewFilterOptions.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconColor: 'var(--color-primary-p3)',
        }
      case TPodsViewFilterOptions.SUCCEEDED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color-green-g1)',
        }
      case TCommonStatuses.DISCONNECTED:
        return {
          iconTypeValue: 'warning',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.PROVISIONED:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color-yellow-y1)',
        }
      case TCommonStatuses.ACTIVE:
      case TCommonStatuses.COMPLETED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color-green-g1)',
        }
      case TCommonStatuses.FINISHED:
      case TCommonStatuses.FAILED:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.EXPIRED:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color-naturals-n10)',
        }
      case TCommonStatuses.REVOKED:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-naturals-n10)',
        }
      case TCommonStatuses.ERROR:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case NodesViewFilterOptions.NOT_READY:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.STOPPING:
      case TCommonStatuses.WAITING:
      case TCommonStatuses.LOADING:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color-primary-p3)',
        }
      case TCommonStatuses.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconColor: 'var(--color-primary-p3)',
        }
      case TCommonStatuses.HEALTH_UNKNOWN:
        return {
          iconTypeValue: 'question',
          iconColor: 'var(--color-naturals-n9)',
        }
      case TCommonStatuses.PROVISION_FAILED:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.PROVISIONING:
        return {
          iconTypeValue: 'loading',
          iconColor: 'var(--color-yellow-y1)',
        }
      case TCommonStatuses.DEPROVISIONING:
        return {
          iconTypeValue: 'delete',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.HEALTHY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color--green-g1)',
        }
      case TCommonStatuses.UNHEALTHY:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.ENABLED:
      case TCommonStatuses.ON:
        return {
          iconTypeValue: 'dot',
          iconColor: 'var(--color--green-g1)',
        }
      case TCommonStatuses.DISABLED:
      case TCommonStatuses.OFF:
        return {
          iconTypeValue: 'dot',
          iconColor: 'var(--color-naturals-n9)',
        }
      case TCommonStatuses.TRUE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color-naturals-n9)',
        }
      case TCommonStatuses.FALSE:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-naturals-n9)',
        }
      case TCommonStatuses.UP_TO_DATE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color--green-g1)',
        }
      case TCommonStatuses.OUTDATED:
        return {
          iconTypeValue: 'error',
          iconColor: 'var(--color-red-r1)',
        }
      case TCommonStatuses.APPLIED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconColor: 'var(--color--green-g1)',
        }
      case TCommonStatuses.PENDING:
        return {
          iconTypeValue: 'time',
          iconColor: 'var(--color--yellow-y1)',
        }
      case TCommonStatuses.AWAITING_CONNECTION:
        return {
          iconTypeValue: 'question',
          iconColor: 'var(--color-naturals-n9)',
        }
      default:
        return {
          iconTypeValue: 'unknown',
          iconColor: 'var(--color-primary-p3)',
        }
    }
  }

  return {}
})

const iconColor = computed(() => {
  if (iconType.value) {
    switch (iconType.value) {
      case 'check-in-circle-classic':
        return { color: 'var(--color--green-g1)' }
      case 'loading':
        return { color: 'var(--color--yellow-y1)' }
      case 'time':
        return { color: 'var(--color-naturals-n9)' }
      case 'refresh':
        return { color: 'var(--color-blue-b1)' }
      default:
        return { color: 'var(--color-naturals-n9)' }
    }
  }

  return {}
})
</script>

<template>
  <div class="label">
    <TIcon
      class="label-icon"
      :style="{
        fill: !!color ? color : iconData?.iconColor || iconColor.color,
      }"
      :icon="!!iconType ? iconType : iconData?.iconTypeValue"
    />
    <span
      v-if="title"
      class="label-title"
      :style="{
        color: !!color ? color : iconData?.iconColor || iconColor.color,
      }"
    >
      {{ title }}
    </span>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

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
