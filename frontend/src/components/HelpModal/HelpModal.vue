<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SupportSpec } from '@/api/omni/specs/virtual.pb'
import { SupportID, SupportType, VirtualNamespace } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import Modal from '@/components/Modals/Modal.vue'
import { getDocsLink } from '@/methods'
import { useResourceGet } from '@/methods/useResourceGet'

const { data: support } = useResourceGet<SupportSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SupportType,
    id: SupportID,
  },
}))

const open = defineModel<boolean>('open', { default: false })

const officeHours = computed(() => support.value?.spec.office_hours)

const googleCalendarUrl = computed(() => {
  const oh = officeHours.value
  const p = new URLSearchParams({
    action: 'TEMPLATE',
    text: oh?.summary ?? '',
    dates: `${oh?.dtstart_utc ?? ''}/${oh?.dtend_utc ?? ''}`,
    details: oh?.description ?? '',
    location: oh?.meet_url ?? '',
    recur: `RRULE:${oh?.rrule ?? ''}`,
  })
  return `https://calendar.google.com/calendar/render?${p.toString()}`
})

const icsUrl = computed(() => {
  const oh = officeHours.value
  const icsContent = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Sidero Labs//Omni//EN',
    'CALSCALE:GREGORIAN',
    'BEGIN:VEVENT',
    `DTSTART;TZID=${oh?.dtstart_tzid ?? ''}`,
    `DTEND;TZID=${oh?.dtend_tzid ?? ''}`,
    `RRULE:${oh?.rrule ?? ''}`,
    `SUMMARY:${oh?.summary ?? ''}`,
    `DESCRIPTION:${(oh?.description ?? '').replace(/,/g, '\\,')}`,
    `URL:${oh?.meet_url ?? ''}`,
    'END:VEVENT',
    'END:VCALENDAR',
  ].join('\r\n')
  return `data:text/calendar;charset=utf-8,${encodeURIComponent(icsContent)}`
})
</script>

<template>
  <Modal v-model:open="open" cancel-label="Close" title="Get support">
    <template #description>Various ways to get help with Omni and Talos</template>

    <div class="flex w-105 flex-col gap-2.5">
      <a
        v-if="support?.spec.support_enabled"
        href="https://support.siderolabs.com/"
        target="_blank"
        rel="noopener noreferrer"
        class="group flex items-center gap-compact rounded-sm border border-border-default bg-surface-card p-compact transition-colors hover:bg-surface-hover"
      >
        <div
          class="flex size-9 shrink-0 items-center justify-center rounded-sm bg-accent-subtle text-accent-text"
        >
          <TIcon class="size-5" icon="lifebuoy" />
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span
            class="text-sm font-medium text-content-default transition-colors group-hover:text-content-emphasis"
          >
            Support Portal
          </span>
          <span class="text-xs text-content-muted">
            File a ticket and get help from the Sidero Labs team
          </span>
        </div>
        <TIcon
          class="size-4 shrink-0 text-content-muted transition-colors group-hover:text-content-muted"
          icon="external-link"
        />
      </a>

      <a
        :href="getDocsLink('omni')"
        target="_blank"
        rel="noopener noreferrer"
        class="group flex items-center gap-compact rounded-sm border border-border-default bg-surface-card p-compact transition-colors hover:bg-surface-hover"
      >
        <div
          class="flex size-9 shrink-0 items-center justify-center rounded-sm bg-surface-inert text-content-secondary"
        >
          <TIcon class="size-5" icon="documentation" />
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span
            class="text-sm font-medium text-content-default transition-colors group-hover:text-content-emphasis"
          >
            Documentation
          </span>
          <span class="text-xs text-content-muted">Guides, API references, and tutorials</span>
        </div>
        <TIcon
          class="size-4 shrink-0 text-content-muted transition-colors group-hover:text-content-muted"
          icon="external-link"
        />
      </a>

      <div
        class="flex items-start gap-compact rounded-sm border border-border-default bg-surface-card p-compact"
      >
        <div
          class="flex size-9 shrink-0 items-center justify-center rounded-sm bg-surface-inert text-content-secondary"
        >
          <TIcon class="size-5" icon="code-bracket" />
        </div>
        <div class="flex flex-col gap-micro">
          <span class="text-sm font-medium text-content-default">GitHub Issues</span>
          <span class="text-xs text-content-muted">Report a bug or request a feature</span>
          <div class="mt-micro flex items-center gap-tight text-xs">
            <a
              href="https://github.com/siderolabs/omni/issues"
              target="_blank"
              rel="noopener noreferrer"
              class="link-primary"
            >
              Omni
            </a>
            <span class="text-content-disabled">·</span>
            <a
              href="https://github.com/siderolabs/talos/issues"
              target="_blank"
              rel="noopener noreferrer"
              class="link-primary"
            >
              Talos
            </a>
            <span class="text-content-disabled">·</span>
            <a
              href="https://github.com/siderolabs/docs/issues"
              target="_blank"
              rel="noopener noreferrer"
              class="link-primary"
            >
              Docs
            </a>
          </div>
        </div>
      </div>

      <a
        :href="getDocsLink('talos', '/overview/what-is-talos#community-&-support')"
        target="_blank"
        rel="noopener noreferrer"
        class="group flex items-center gap-compact rounded-sm border border-border-default bg-surface-card p-compact transition-colors hover:bg-surface-hover"
      >
        <div
          class="flex size-9 shrink-0 items-center justify-center rounded-sm bg-surface-inert text-content-secondary"
        >
          <TIcon class="size-5" icon="users" />
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span
            class="text-sm font-medium text-content-default transition-colors group-hover:text-content-emphasis"
          >
            Community
          </span>
          <span class="text-xs text-content-muted">Chat with the community on Slack and more</span>
        </div>
        <TIcon
          class="size-4 shrink-0 text-content-muted transition-colors group-hover:text-content-muted"
          icon="external-link"
        />
      </a>

      <div
        class="flex items-start gap-compact rounded-sm border border-border-default bg-surface-card p-compact"
      >
        <div
          class="flex size-9 shrink-0 items-center justify-center rounded-sm bg-surface-inert text-content-secondary"
        >
          <TIcon class="size-5" icon="calendar" />
        </div>
        <div class="flex flex-1 flex-col gap-micro">
          <span class="text-sm font-medium text-content-default">Office Hours</span>
          <span class="text-xs text-content-muted">
            Monthly community call with Sidero Labs engineers
          </span>
          <div class="mt-micro flex items-center gap-snug">
            <a
              :href="getDocsLink('talos', '/overview/what-is-talos#office-hours')"
              target="_blank"
              rel="noopener noreferrer"
              class="link-primary text-xs"
            >
              Learn more
            </a>
            <TButton
              is="a"
              :href="googleCalendarUrl"
              icon="calendar"
              icon-position="left"
              size="sm"
              target="_blank"
              rel="noopener noreferrer"
              variant="secondary"
            >
              Google Calendar
            </TButton>
            <TButton
              is="a"
              :href="icsUrl"
              download="sidero-office-hours.ics"
              icon="arrow-down-tray"
              icon-position="left"
              size="sm"
              variant="secondary"
            >
              .ics
            </TButton>
          </div>
        </div>
      </div>
    </div>
  </Modal>
</template>
