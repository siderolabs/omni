<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  DateRangePickerArrow,
  DateRangePickerCalendar,
  DateRangePickerCell,
  DateRangePickerCellTrigger,
  DateRangePickerContent,
  DateRangePickerField,
  DateRangePickerGrid,
  DateRangePickerGridBody,
  DateRangePickerGridHead,
  DateRangePickerGridRow,
  DateRangePickerHeadCell,
  DateRangePickerHeader,
  DateRangePickerHeading,
  DateRangePickerInput,
  DateRangePickerNext,
  DateRangePickerPrev,
  DateRangePickerRoot,
  type DateRangePickerRootEmits,
  type DateRangePickerRootProps,
  DateRangePickerTrigger,
  Label,
  useForwardPropsEmits,
} from 'reka-ui'
import { useId } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'

interface Props extends DateRangePickerRootProps {
  title: string
  hiddenTitle?: boolean
  inlineTitle?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<DateRangePickerRootEmits>()

const forwardedProps = reactiveOmit(props, 'title', 'hiddenTitle')
const forwarded = useForwardPropsEmits(forwardedProps, emit)

const id = useId()
</script>

<template>
  <div class="inline-flex gap-2" :class="inlineTitle ? 'items-center' : 'flex-col'">
    <Label class="text-sm text-content-emphasis" :class="{ 'sr-only': hiddenTitle }" :for="id">
      {{ title }}
    </Label>

    <DateRangePickerRoot v-bind="forwarded" :id>
      <DateRangePickerField
        v-slot="{ segments }"
        class="flex items-center rounded border border-border-strong bg-surface-raised p-1 text-center text-sm text-content-default select-none data-invalid:border-status-danger-default"
      >
        <template v-for="item in segments.start" :key="item.part">
          <DateRangePickerInput v-if="item.part === 'literal'" :part="item.part" type="start">
            {{ item.value }}
          </DateRangePickerInput>
          <DateRangePickerInput
            v-else
            :part="item.part"
            class="rounded p-0.5 focus:bg-surface-inert focus:outline-none data-placeholder:text-content-muted"
            type="start"
          >
            {{ item.value }}
          </DateRangePickerInput>
        </template>
        <span class="mx-2 text-content-muted">-</span>
        <template v-for="item in segments.end" :key="item.part">
          <DateRangePickerInput v-if="item.part === 'literal'" :part="item.part" type="end">
            {{ item.value }}
          </DateRangePickerInput>
          <DateRangePickerInput
            v-else
            :part="item.part"
            class="rounded p-0.5 focus:bg-surface-inert focus:outline-none data-placeholder:text-content-muted"
            type="end"
          >
            {{ item.value }}
          </DateRangePickerInput>
        </template>

        <DateRangePickerTrigger
          class="ml-4 rounded p-1 text-content-muted hover:text-content-default focus:outline-none"
        >
          <TIcon icon="calendar" class="h-4 w-4" />
        </DateRangePickerTrigger>
      </DateRangePickerField>

      <DateRangePickerContent
        :side-offset="4"
        class="data-[state=open]:data-[side=bottom]:animate-slideUpAndFade data-[state=open]:data-[side=left]:animate-slideRightAndFade data-[state=open]:data-[side=right]:animate-slideLeftAndFade data-[state=open]:data-[side=top]:animate-slideDownAndFade z-100 rounded border border-border-strong bg-surface-raised shadow-lg will-change-[transform,opacity]"
      >
        <DateRangePickerArrow class="fill-surface-raised stroke-surface-inert" />
        <DateRangePickerCalendar v-slot="{ weekDays, grid }" class="p-4">
          <DateRangePickerHeader class="flex items-center justify-between">
            <DateRangePickerPrev
              class="inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent bg-transparent text-content-muted hover:border-border-strong hover:bg-surface-inert hover:text-content-default focus:outline-none active:bg-surface-hover"
            >
              <TIcon icon="chevron-left" class="h-4 w-4" />
            </DateRangePickerPrev>

            <DateRangePickerHeading class="text-sm font-medium text-content-default" />
            <DateRangePickerNext
              class="inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent bg-transparent text-content-muted hover:border-border-strong hover:bg-surface-inert hover:text-content-default focus:outline-none active:bg-surface-hover"
            >
              <TIcon icon="chevron-right" class="h-4 w-4" />
            </DateRangePickerNext>
          </DateRangePickerHeader>
          <div class="flex flex-col space-y-4 pt-4 sm:flex-row sm:space-y-0 sm:space-x-4">
            <DateRangePickerGrid
              v-for="month in grid"
              :key="month.value.toString()"
              class="w-full border-collapse space-y-1 select-none"
            >
              <DateRangePickerGridHead>
                <DateRangePickerGridRow class="mb-1 flex w-full justify-between">
                  <DateRangePickerHeadCell
                    v-for="day in weekDays"
                    :key="day"
                    class="w-8 rounded text-xs font-normal! text-content-muted"
                  >
                    {{ day }}
                  </DateRangePickerHeadCell>
                </DateRangePickerGridRow>
              </DateRangePickerGridHead>
              <DateRangePickerGridBody>
                <DateRangePickerGridRow
                  v-for="(weekDates, index) in month.rows"
                  :key="`weekDate-${index}`"
                  class="flex w-full"
                >
                  <DateRangePickerCell
                    v-for="weekDate in weekDates"
                    :key="weekDate.toString()"
                    :date="weekDate"
                  >
                    <DateRangePickerCellTrigger
                      :day="weekDate"
                      :month="month.value"
                      class="relative flex h-8 w-8 items-center justify-center rounded text-sm font-normal whitespace-nowrap text-content-default outline-none before:absolute before:top-1.25 before:hidden before:h-1 before:w-1 before:rounded-full before:bg-accent-fill hover:bg-surface-inert focus:bg-surface-inert data-highlighted:bg-accent-fill/25 data-outside-view:text-content-muted data-selected:bg-accent-fill! data-selected:text-content-emphasis data-today:before:block data-unavailable:pointer-events-none data-unavailable:text-content-muted data-unavailable:line-through"
                    />
                  </DateRangePickerCell>
                </DateRangePickerGridRow>
              </DateRangePickerGridBody>
            </DateRangePickerGrid>
          </div>
        </DateRangePickerCalendar>
      </DateRangePickerContent>
    </DateRangePickerRoot>
  </div>
</template>
