<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="watch">
    <div
      v-if="spinner && loading"
      class="flex flex-row justify-center items-center w-full h-full"
    >
      <t-spinner class="loading-spinner"/>
    </div>
    <template v-else-if="errorsAlert && err">
      <t-alert v-if="!$slots.error"
        title="Failed to Fetch Data"
        type="error"
      >
        {{ err }}.
      </t-alert>
      <slot name="error" v-else err="err"/>
    </template>
    <template v-else-if="noRecordsAlert && items.length == 0">
      <t-alert
        v-if="!$slots.norecords"
        type="info"
        title="No Records"
        >No entries of the requested resource type are found on the
        server.</t-alert
      >
      <slot name="norecords"/>
    </template>
    <div class="wrapper" v-show="!loading && !err && (items.length > 0 || !noRecordsAlert) || displayAlways">
      <slot :items="items" :watch="resourceWatch"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import Watch, { WatchJoin, WatchOptions, WatchJoinOptions } from "@/api/watch";
import { ref, toRefs } from "vue";

import TAlert from "@/components/TAlert.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";

type Props = {
  opts: WatchJoinOptions[] | WatchOptions & Object, // & Object is used to make Vue validator happy
  spinner?: boolean,
  noRecordsAlert?: boolean
  errorsAlert?: boolean,
  displayAlways?: boolean,
};

const props = defineProps<Props>();
const resourceWatch: Watch<any> | WatchJoin<any> = props.opts.length ? new WatchJoin(ref([])) : new Watch(ref([]));
const { opts } = toRefs(props);

if (opts.value.length) {
  resourceWatch.setup(opts.value[0], opts.value.slice(1, opts.value.length));
} else {
  resourceWatch.setup(opts);
}

const items = resourceWatch.items;
const err = resourceWatch.err;
const loading = resourceWatch.loading;
</script>

<style scoped>
.watch {
  @apply w-full;
}
.wrapper {
  @apply w-full h-full;
}
.loading-spinner {
  @apply absolute top-2/4 w-6 h-6;
}
</style>
