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
import { EphemeralNamespace, NotificationType } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import HelpModal from '@/components/HelpModal/HelpModal.vue'
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import OngoingTasks from '@/components/OngoingTasks/OngoingTasks.vue'
import ThemeMenu from '@/components/THeader/ThemeMenu.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

interface Props {
  sidebarOpen?: boolean
}

defineProps<Props>()
defineEmits<{ toggleSidebar: [] }>()

const { data } = useResourceWatch<NotificationSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: EphemeralNamespace,
    type: NotificationType,
  },
})

const helpModalOpen = ref(false)
const dismissedNotifications = useLocalStorage<string[]>('_dismissed_header_notifications', [])
const currentOffset = ref(0)

const notifications = computed(() =>
  data.value
    .filter((d) => !dismissedNotifications.value.includes(d.metadata.id!))
    .sort((a, b) => (b.spec.type ?? 0) - (a.spec.type ?? 0)),
)
const currentNotification = computed(() => notifications.value.at(currentOffset.value))

function getIcon(type: NotificationSpecType): IconType {
  switch (type) {
    case NotificationSpecType.ERROR:
      return 'x-circle'
    case NotificationSpecType.WARNING:
      return 'exclamation-triangle'
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
      class="flex h-12 items-center justify-between border-b border-border-default bg-surface-chrome px-3 md:h-13 md:px-6"
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

        <RouterLink to="/" class="flex items-center gap-1 text-lg text-content-default uppercase">
          <TIcon class="t-header-icon size-6" icon="logo" />
          <span class="font-bold">Sidero</span>
          <span>Omni</span>
        </RouterLink>
      </div>

      <div class="flex min-h-12 items-center gap-2 px-6">
        <TButton
          variant="subtle"
          icon="check-circle"
          icon-position="left"
          class="text-content-secondary"
          @click="helpModalOpen = true"
        >
          <span class="max-sm:sr-only">Support</span>
        </TButton>

        <OngoingTasks />

        <ThemeMenu />
      </div>
    </header>

    <div
      v-if="currentNotification"
      class="flex items-center justify-end gap-6 px-6 py-2 transition-colors"
      :class="{
        'bg-status-danger-subtle': currentNotification.spec.type === NotificationSpecType.ERROR,
        'bg-status-warning-subtle': currentNotification.spec.type === NotificationSpecType.WARNING,
        'bg-status-info-subtle': currentNotification.spec.type === NotificationSpecType.INFO,
      }"
    >
      <div class="flex items-center gap-2">
        <TIcon
          class="size-4 shrink-0 transition-colors"
          :icon="getIcon(currentNotification.spec.type!)"
          :class="{
            'text-status-danger-text': currentNotification.spec.type === NotificationSpecType.ERROR,
            'text-status-warning-text':
              currentNotification.spec.type === NotificationSpecType.WARNING,
            'text-status-info-text': currentNotification.spec.type === NotificationSpecType.INFO,
          }"
        />
        <span class="text-xs text-content-emphasis">
          <span class="font-bold">{{ currentNotification.spec.title }}:</span>
          {{ currentNotification.spec.body }}
        </span>
      </div>

      <div class="flex items-center gap-2">
        <TButton
          icon="chevron-left"
          icon-position="left"
          :disabled="currentOffset === 0"
          @click="currentOffset = Math.max(0, currentOffset - 1)"
        >
          <span class="max-sm:sr-only">Previous</span>
        </TButton>

        <TButton
          icon="chevron-right"
          :disabled="currentOffset >= notifications.length - 1"
          @click="currentOffset = Math.min(notifications.length - 1, currentOffset + 1)"
        >
          <span class="max-sm:sr-only">Next</span>
        </TButton>

        <IconButton icon="close" @click="dismissNotification(currentNotification.metadata.id!)" />
      </div>
    </div>

    <HelpModal v-model:open="helpModalOpen" />
  </div>
</template>
