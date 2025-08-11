<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends Resource">
import { computed, ref, toRefs } from 'vue'

import type { Resource } from '@/api/grpc'
import type { WatchJoinOptions, WatchOptions } from '@/api/watch'
import Watch, { WatchJoin } from '@/api/watch'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'

type Props = {
  opts: WatchJoinOptions[] | (WatchOptions & object) | undefined // & Object is used to make Vue validator happy
  spinner?: boolean
  noRecordsAlert?: boolean
  errorsAlert?: boolean
  displayAlways?: boolean
}

const props = defineProps<Props>()
const items = ref<T[]>([])

const watchSingle = !Array.isArray(props.opts) ? new Watch(items) : undefined
const watchJoin = Array.isArray(props.opts) ? new WatchJoin(items) : undefined

const resourceWatch = watchSingle ?? watchJoin

const { opts } = toRefs(props)

if (watchJoin) {
  watchJoin.setup(
    computed(() => {
      if (!opts?.value) {
        return
      }

      return (opts.value as WatchJoinOptions[])[0]
    }),
    computed(() => {
      if (!opts?.value) {
        return
      }

      const o = opts.value as WatchJoinOptions[]

      return o.slice(1, o.length)
    }),
  )
} else if (watchSingle) {
  watchSingle.setup(
    computed(() => {
      if (!opts?.value) {
        return
      }

      return opts.value as WatchOptions
    }),
  )
}

const err = resourceWatch!.err
const loading = resourceWatch!.loading
</script>

<template>
  <div class="watch">
    <div v-if="spinner && loading" class="flex h-full w-full flex-row items-center justify-center">
      <TSpinner class="loading-spinner" />
    </div>
    <template v-else-if="errorsAlert && err">
      <TAlert v-if="!$slots.error" title="Failed to Fetch Data" type="error"> {{ err }}. </TAlert>
      <slot v-else name="error" err="err" />
    </template>
    <template v-else-if="noRecordsAlert && items.length === 0">
      <TAlert v-if="!$slots.norecords" type="info" title="No Records"
        >No entries of the requested resource type are found on the server.</TAlert
      >
      <slot name="norecords" />
    </template>
    <div
      v-show="(!loading && !err && (items.length > 0 || !noRecordsAlert)) || displayAlways"
      class="wrapper"
    >
      <slot :items="items" :watch="resourceWatch" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.watch {
  @apply w-full;
}
.wrapper {
  @apply h-full w-full;
}
.loading-spinner {
  @apply absolute top-2/4 h-6 w-6;
}
</style>
