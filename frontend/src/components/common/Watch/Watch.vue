<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = any">
import type { WatchJoinOptions, WatchOptions } from '@/api/watch'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import TAlert from '@/components/TAlert.vue'

type Props = {
  opts: WatchJoinOptions[] | WatchOptions
  spinner?: boolean
  noRecordsAlert?: boolean
  errorsAlert?: boolean
  displayAlways?: boolean
}

const props = defineProps<Props>()

const { items, err, loading } = useWatch<T, any>(props.opts)
</script>

<template>
  <div v-if="spinner && loading" class="flex size-full items-center justify-center">
    <TSpinner class="size-6" />
  </div>

  <slot v-else-if="errorsAlert && err" name="error" :err="err">
    <TAlert title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
  </slot>

  <slot v-else-if="noRecordsAlert && !items.length" name="norecords">
    <TAlert type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>
  </slot>

  <slot
    v-if="(!loading && !err && (items.length || !noRecordsAlert)) || displayAlways"
    :items="items"
  />
</template>
