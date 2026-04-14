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

// eslint-disable-next-line vue/define-props-destructuring
const props = defineProps<Props>()
const emit = defineEmits<DateRangePickerRootEmits>()

const forwardedProps = reactiveOmit(props, 'title', 'hiddenTitle')
const forwarded = useForwardPropsEmits(forwardedProps, emit)

const id = useId()
</script>

<template>
  <div class="inline-flex gap-2" :class="inlineTitle ? 'items-center' : 'flex-col'">
    <Label class="text-sm text-naturals-n14" :class="{ 'sr-only': hiddenTitle }" :for="id">
      {{ title }}
    </Label>

    <DateRangePickerRoot v-bind="forwarded" :id>
      <DateRangePickerField
        v-slot="{ segments }"
        class="flex items-center rounded border border-naturals-n6 bg-naturals-n3 p-1 text-center text-sm text-naturals-n13 select-none data-invalid:border-red-r1"
      >
        <template v-for="item in segments.start" :key="item.part">
          <DateRangePickerInput v-if="item.part === 'literal'" :part="item.part" type="start">
            {{ item.value }}
          </DateRangePickerInput>
          <DateRangePickerInput
            v-else
            :part="item.part"
            class="rounded p-0.5 focus:bg-naturals-n5 focus:outline-none data-placeholder:text-naturals-n8"
            type="start"
          >
            {{ item.value }}
          </DateRangePickerInput>
        </template>
        <span class="mx-2 text-naturals-n8">-</span>
        <template v-for="item in segments.end" :key="item.part">
          <DateRangePickerInput v-if="item.part === 'literal'" :part="item.part" type="end">
            {{ item.value }}
          </DateRangePickerInput>
          <DateRangePickerInput
            v-else
            :part="item.part"
            class="rounded p-0.5 focus:bg-naturals-n5 focus:outline-none data-placeholder:text-naturals-n8"
            type="end"
          >
            {{ item.value }}
          </DateRangePickerInput>
        </template>

        <DateRangePickerTrigger
          class="ml-4 rounded p-1 text-naturals-n10 hover:text-naturals-n13 focus:outline-none"
        >
          <TIcon icon="calendar" class="h-4 w-4" />
        </DateRangePickerTrigger>
      </DateRangePickerField>

      <DateRangePickerContent
        :side-offset="4"
        class="data-[state=open]:data-[side=bottom]:animate-slideUpAndFade data-[state=open]:data-[side=left]:animate-slideRightAndFade data-[state=open]:data-[side=right]:animate-slideLeftAndFade data-[state=open]:data-[side=top]:animate-slideDownAndFade z-100 rounded border border-naturals-n6 bg-naturals-n3 shadow-lg will-change-[transform,opacity]"
      >
        <DateRangePickerArrow class="fill-naturals-n3 stroke-naturals-n6" />
        <DateRangePickerCalendar v-slot="{ weekDays, grid }" class="p-4">
          <DateRangePickerHeader class="flex items-center justify-between">
            <DateRangePickerPrev
              class="inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent bg-transparent text-naturals-n10 hover:border-naturals-n6 hover:bg-naturals-n5 hover:text-naturals-n13 focus:outline-none active:bg-naturals-n4"
            >
              <TIcon icon="arrow-left" class="h-4 w-4" />
            </DateRangePickerPrev>

            <DateRangePickerHeading class="text-sm font-medium text-naturals-n13" />
            <DateRangePickerNext
              class="inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent bg-transparent text-naturals-n10 hover:border-naturals-n6 hover:bg-naturals-n5 hover:text-naturals-n13 focus:outline-none active:bg-naturals-n4"
            >
              <TIcon icon="arrow-right" class="h-4 w-4" />
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
                    class="w-8 rounded text-xs font-normal! text-naturals-n9"
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
                      class="relative flex h-8 w-8 items-center justify-center rounded text-sm font-normal whitespace-nowrap text-naturals-n13 outline-none before:absolute before:top-1.25 before:hidden before:h-1 before:w-1 before:rounded-full before:bg-primary-p3 hover:bg-naturals-n5 focus:bg-naturals-n5 data-highlighted:bg-primary-p4/25 data-outside-view:text-naturals-n7 data-selected:bg-primary-p4! data-selected:text-naturals-n14 data-today:before:block data-unavailable:pointer-events-none data-unavailable:text-naturals-n7 data-unavailable:line-through"
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
