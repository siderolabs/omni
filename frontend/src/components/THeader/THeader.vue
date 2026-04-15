<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useLocalStorage } from '@vueuse/core'
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type NotificationSpec, NotificationSpecType } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, NotificationType } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import OngoingTasks from '@/components/OngoingTasks/OngoingTasks.vue'
import { getDocsLink } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'

interface Props {
  sidebarOpen?: boolean
}

defineProps<Props>()
defineEmits<{ toggleSidebar: [] }>()

const { data } = useResourceWatch<NotificationSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: NotificationType,
  },
})

const dismissedNotifications = useLocalStorage<string[]>('_dismissed_header_notifications', [])
const currentOffset = ref(0)

const notifications = computed(() =>
  data.value
    .filter((d) => !dismissedNotifications.value.includes(d.metadata.id!))
    .sort((a, b) => (b.spec.type ?? 0) - (a.spec.type ?? 0)),
)
const currentNotification = computed(() => notifications.value.at(currentOffset.value))

const docsLink = getDocsLink('omni')

function getIcon(type: NotificationSpecType): IconType {
  switch (type) {
    case NotificationSpecType.ERROR:
      return 'error'
    case NotificationSpecType.WARNING:
      return 'warning'
    case NotificationSpecType.INFO:
    default:
      return 'info'
  }
}

function dismissNotification(id: string) {
  dismissedNotifications.value.push(id)
  currentOffset.value = Math.max(0, currentOffset.value - 1)
}
</script>

<template>
  <div class="flex flex-col">
    <header
      class="flex h-12 items-center justify-between border-b border-naturals-n4 bg-naturals-n1 px-3 md:h-13 md:px-6"
    >
      <div class="flex items-center gap-4">
        <TButton
          class="relative size-6 p-0! md:hidden"
          aria-controls="sidebar"
          :aria-expanded="sidebarOpen"
          @click="$emit('toggleSidebar')"
        >
          <TIcon
            aria-label="open sidebar"
            :aria-hidden="sidebarOpen"
            icon="hamburger"
            class="absolute inset-0 m-auto size-4 transition-all"
            :class="sidebarOpen ? 'rotate-90 opacity-0' : 'opacity-100'"
          />
          <TIcon
            aria-label="close sidebar"
            :aria-hidden="!sidebarOpen"
            icon="close"
            class="absolute inset-0 m-auto size-4 transition-all"
            :class="sidebarOpen ? 'opacity-100' : '-rotate-90 opacity-0'"
          />
        </TButton>

        <RouterLink to="/" class="flex items-center gap-1 text-lg text-naturals-n13 uppercase">
          <TIcon class="t-header-icon size-6" icon="logo" />
          <span class="font-bold">Sidero</span>
          <span>Omni</span>
        </RouterLink>
      </div>

      <div class="flex min-h-12 items-center gap-2 px-6 text-naturals-n11">
        <a
          :href="docsLink"
          target="_blank"
          rel="noopener noreferrer"
          class="flex gap-1 transition-colors hover:text-naturals-n14"
        >
          <TIcon class="size-4" icon="info" />
          <span class="truncate text-xs max-sm:hidden">Documentation</span>
        </a>

        <a
          href="https://github.com/siderolabs/omni/issues"
          target="_blank"
          rel="noopener noreferrer"
          class="flex gap-1 transition-colors hover:text-naturals-n14"
        >
          <TIcon class="size-4" icon="check-in-circle" />
          <span class="truncate text-xs max-sm:hidden">Report an issue</span>
        </a>

        <OngoingTasks />
      </div>
    </header>

    <div
      v-if="currentNotification"
      class="flex items-center justify-end gap-6 px-6 py-2 transition-colors"
      :class="{
        'bg-red-r1/15': currentNotification.spec.type === NotificationSpecType.ERROR,
        'bg-yellow-y1/15': currentNotification.spec.type === NotificationSpecType.WARNING,
        'bg-blue-b1/15': currentNotification.spec.type === NotificationSpecType.INFO,
      }"
    >
      <div class="flex items-center gap-2">
        <TIcon
          class="size-4 shrink-0 transition-colors"
          :icon="getIcon(currentNotification.spec.type!)"
          :class="{
            'text-red-r1': currentNotification.spec.type === NotificationSpecType.ERROR,
            'text-yellow-y1': currentNotification.spec.type === NotificationSpecType.WARNING,
            'text-blue-b1': currentNotification.spec.type === NotificationSpecType.INFO,
          }"
        />
        <span class="text-xs text-naturals-n14">
          <span class="font-bold">{{ currentNotification.spec.title }}:</span>
          {{ currentNotification.spec.body }}
        </span>
      </div>

      <div class="flex items-center gap-2">
        <TButton
          icon="arrow-left"
          icon-position="left"
          :disabled="currentOffset === 0"
          @click="currentOffset = Math.max(0, currentOffset - 1)"
        >
          <span class="max-sm:sr-only">Previous</span>
        </TButton>

        <TButton
          icon="arrow-right"
          :disabled="currentOffset >= notifications.length - 1"
          @click="currentOffset = Math.min(notifications.length - 1, currentOffset + 1)"
        >
          <span class="max-sm:sr-only">Next</span>
        </TButton>

        <IconButton icon="close" @click="dismissNotification(currentNotification.metadata.id!)" />
      </div>
    </div>
  </div>
</template>
