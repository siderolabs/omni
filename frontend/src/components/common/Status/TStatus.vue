<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { NodesViewFilterOptions, TCommonStatuses, TPodsViewFilterOptions } from '@/constants'

type Props = {
  title?: string
}

const { title } = defineProps<Props>()

const iconData = computed((): { iconClass?: string; iconTypeValue?: IconType } => {
  if (title) {
    switch (title) {
      case TPodsViewFilterOptions.RUNNING:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case NodesViewFilterOptions.READY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TPodsViewFilterOptions.PENDING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-yellow-y1',
        }
      case TPodsViewFilterOptions.FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case TPodsViewFilterOptions.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-primary-p3',
        }
      case TPodsViewFilterOptions.SUCCEEDED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.DISCONNECTED:
        return {
          iconTypeValue: 'warning',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.PROVISIONED:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-yellow-y1',
        }
      case TCommonStatuses.ACTIVE:
      case TCommonStatuses.COMPLETED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.FINISHED:
      case TCommonStatuses.FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.EXPIRED:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-naturals-n10',
        }
      case TCommonStatuses.REVOKED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-naturals-n10',
        }
      case TCommonStatuses.ERROR:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case NodesViewFilterOptions.NOT_READY:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.STOPPING:
      case TCommonStatuses.WAITING:
      case TCommonStatuses.LOADING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-primary-p3',
        }
      case TCommonStatuses.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-primary-p3',
        }
      case TCommonStatuses.HEALTH_UNKNOWN:
        return {
          iconTypeValue: 'question',
          iconClass: 'text-naturals-n9',
        }
      case TCommonStatuses.PROVISION_FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.PROVISIONING:
        return {
          iconTypeValue: 'loading',
          iconClass: 'text-yellow-y1',
        }
      case TCommonStatuses.DEPROVISIONING:
        return {
          iconTypeValue: 'delete',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.HEALTHY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.UNHEALTHY:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.ENABLED:
      case TCommonStatuses.ON:
        return {
          iconTypeValue: 'dot',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.DISABLED:
      case TCommonStatuses.OFF:
        return {
          iconTypeValue: 'dot',
          iconClass: 'text-naturals-n9',
        }
      case TCommonStatuses.TRUE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-naturals-n9',
        }
      case TCommonStatuses.FALSE:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-naturals-n9',
        }
      case TCommonStatuses.UP_TO_DATE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.OUTDATED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-red-r1',
        }
      case TCommonStatuses.APPLIED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-green-g1',
        }
      case TCommonStatuses.PENDING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-yellow-y1',
        }
      case TCommonStatuses.AWAITING_CONNECTION:
        return {
          iconTypeValue: 'question',
          iconClass: 'text-naturals-n9',
        }
      default:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-primary-p3',
        }
    }
  }

  return {}
})
</script>

<template>
  <div class="flex items-center gap-1">
    <TIcon
      class="size-4"
      :class="iconData.iconClass"
      :icon="iconData.iconTypeValue ?? 'action-horizontal'"
    />
    <span v-if="title" class="text-xs" :class="iconData.iconClass">
      {{ title }}
    </span>
  </div>
</template>
