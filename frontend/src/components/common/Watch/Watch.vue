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

<script setup lang="ts" generic="T extends Resource">
import Watch, { WatchJoin, WatchOptions, WatchJoinOptions } from "@/api/watch";
import { computed, ref, toRefs } from "vue";

import TAlert from "@/components/TAlert.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import { Resource } from "@/api/grpc";

type Props = {
  opts: WatchJoinOptions[] | WatchOptions & object | undefined, // & Object is used to make Vue validator happy
  spinner?: boolean,
  noRecordsAlert?: boolean
  errorsAlert?: boolean,
  displayAlways?: boolean,
};

const props = defineProps<Props>();
const items = ref<T[]>([]);

const watchSingle: Watch<T> | undefined = (props.opts as { length: number })?.length == undefined ? new Watch<T>(items) : undefined;
const watchJoin: WatchJoin<T> | undefined = (props.opts as { length: number })?.length != undefined ? new WatchJoin<T>(items) : undefined;

const resourceWatch = watchSingle ?? watchJoin;

const { opts } = toRefs(props);

if (watchJoin) {
  watchJoin.setup(
    computed(() => {
      if (!opts?.value) {
        return;
      }

      return (opts.value as WatchJoinOptions[])[0];
    }),
    computed(() => {
      if (!opts?.value) {
        return;
      }

      const o = opts.value as WatchJoinOptions[];

      return o.slice(1, o.length);
    })
  );
} else if (watchSingle) {
  watchSingle.setup(computed(() => {
    if (!opts?.value) {
      return;
    }

    return opts.value as WatchOptions
  }));
}

const err = resourceWatch!.err;
const loading = resourceWatch!.loading;
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
