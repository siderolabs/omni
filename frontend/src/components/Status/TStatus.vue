<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
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
          iconClass: 'text-status-success-text',
        }
      case NodesViewFilterOptions.READY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TPodsViewFilterOptions.PENDING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-status-warning-text',
        }
      case TPodsViewFilterOptions.FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case TPodsViewFilterOptions.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-accent-text',
        }
      case TPodsViewFilterOptions.SUCCEEDED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.DISCONNECTED:
        return {
          iconTypeValue: 'warning',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.PROVISIONED:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-status-warning-text',
        }
      case TCommonStatuses.ACTIVE:
      case TCommonStatuses.COMPLETED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.FINISHED:
        return {
          iconTypeValue: 'stop-circle',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.EXPIRED:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.REVOKED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.ERROR:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case NodesViewFilterOptions.NOT_READY:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.STOPPING:
      case TCommonStatuses.WAITING:
      case TCommonStatuses.LOADING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-accent-text',
        }
      case TCommonStatuses.UNKNOWN:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-accent-text',
        }
      case TCommonStatuses.HEALTH_UNKNOWN:
        return {
          iconTypeValue: 'question',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.PROVISION_FAILED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.PROVISIONING:
        return {
          iconTypeValue: 'loading',
          iconClass: 'text-status-warning-text',
        }
      case TCommonStatuses.DEPROVISIONING:
        return {
          iconTypeValue: 'delete',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.HEALTHY:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.UNHEALTHY:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.ENABLED:
      case TCommonStatuses.ON:
        return {
          iconTypeValue: 'dot',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.DISABLED:
      case TCommonStatuses.OFF:
        return {
          iconTypeValue: 'dot',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.TRUE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.FALSE:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-content-muted',
        }
      case TCommonStatuses.UP_TO_DATE:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.OUTDATED:
        return {
          iconTypeValue: 'error',
          iconClass: 'text-status-danger-text',
        }
      case TCommonStatuses.APPLIED:
        return {
          iconTypeValue: 'check-in-circle-classic',
          iconClass: 'text-status-success-text',
        }
      case TCommonStatuses.PENDING:
        return {
          iconTypeValue: 'time',
          iconClass: 'text-status-warning-text',
        }
      case TCommonStatuses.AWAITING_CONNECTION:
        return {
          iconTypeValue: 'question',
          iconClass: 'text-content-muted',
        }
      default:
        return {
          iconTypeValue: 'unknown',
          iconClass: 'text-accent-text',
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
