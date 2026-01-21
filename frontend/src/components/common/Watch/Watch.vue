<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script
  setup
  lang="ts"
  generic="
    TSpec = any,
    TStatus = any,
    TOptions extends WatchJoinOptions[] | WatchOptions = WatchJoinOptions[] | WatchOptions
  "
>
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { WatchJoinOptions, WatchOptions, WatchOptionsSingle } from '@/api/watch'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

type Props = {
  opts: TOptions
  spinner?: boolean
  noRecordsAlert?: boolean
  errorsAlert?: boolean
  displayAlways?: boolean
}

const props = defineProps<Props>()

defineSlots<{
  error(props: { err: string | null }): unknown
  norecords(): unknown
  default(props: {
    data: TOptions extends WatchOptionsSingle
      ? Resource<TSpec, TStatus> | undefined
      : Resource<TSpec, TStatus>[]
  }): unknown
}>()

const { data, err, loading } = useResourceWatch<TSpec, TStatus>(() => props.opts)

const hasData = computed(() => (Array.isArray(data.value) ? !!data.value.length : !!data.value))
</script>

<template>
  <div v-if="spinner && loading" class="flex size-full items-center justify-center">
    <TSpinner class="size-6" />
  </div>

  <slot v-else-if="errorsAlert && err" name="error" :err="err">
    <TAlert title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
  </slot>

  <slot v-else-if="noRecordsAlert && !hasData" name="norecords">
    <TAlert type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>
  </slot>

  <!-- No idea how to type data correctly here but it works if you ✨believe✨ -->
  <slot
    v-if="(!loading && !err && (hasData || !noRecordsAlert)) || displayAlways"
    :data="data as any"
  />
</template>
