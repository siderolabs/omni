<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <slot name="header" :itemsCount="itemsCount" :filtered="searchState.searchFor?.length || searchState.selectors?.length"/>
    <div class="flex flex-col gap-4">
      <div class="flex gap-2" v-if="pagination || search || itemsPerPage?.length > 1">
        <slot v-if="$slots.input" name="input"/>
        <t-input v-else-if="search" class="flex-1" icon="search" v-model="filterValueInternal"/>
        <div class="flex-1" v-else/>
        <t-select-list
            @checkedValue="(value: string) => { selectedSortOption = value }"
            v-if="sortOptions"
            title="Sort by"
            :defaultValue="selectedSortOption || ''"
            :values="sortOptionsVariants"
          />
        <t-select-list
            @checkedValue="(value: number) => { selectedItemsPerPage = value; currentPage = 1 }"
            v-if="itemsPerPage?.length > 1 && pagination"
            title="Items per Page"
            :defaultValue="selectedItemsPerPage"
            :values="itemsPerPage"
          />
      </div>
      <div class="flex-1">
        <div
          v-if="loading"
          class="flex flex-row justify-center items-center w-full h-full"
        >
          <t-spinner class="loading-spinner"/>
        </div>
        <template v-else-if="err">
          <t-alert v-if="!$slots.error"
            title="Failed to Fetch Data"
            type="error"
          >
            {{ err }}.
          </t-alert>
          <slot name="error" v-else err="err"/>
        </template>
        <template v-else-if="items?.length === 0">
          <t-alert
            v-if="!$slots.norecords"
            type="info"
            title="No Records"
            >No entries of the requested resource type are found on the
            server.</t-alert
          >
          <slot name="norecords"/>
        </template>
        <div class="w-full h-full" v-show="!loading && !err && (items?.length > 0)">
          <slot :items="items" :watch="watch" :searchQuery="searchQuery"/>
        </div>
      </div>
      <div class="flex items-center justify-end gap-2" v-if="showPageSelector">
        <t-icon icon="arrow-left" class="pagination-icon" :class="{ 'pagination-icon-disabled': currentPage === 1 }"
          @click="prevPage" />
        <div class="pagination-pages">
          <span @click="() => openPage(item)" class="pagination-page-number" :class="{
                'pagination-page-number-active': item === currentPage,
                unhovered: item === dots,
              }" v-for="item, index in paginationRange ?? []" :key="index">
            {{ item }}</span>
        </div>
        <t-icon icon="arrow-right" class="pagination-icon"
          :class="{ 'pagination-icon-disabled': currentPage === totalPageCount }" @click="nextPage"/>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts" generic="T extends Resource">
import { Resource } from "@/api/grpc";
import Watch, { WatchJoin, WatchJoinOptions, WatchOptions } from "@/api/watch";
import TInput from "@/components/common/TInput/TInput.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TAlert from "@/components/TAlert.vue";

import { computed, Ref, ref, toRefs, watch as vueWatch } from "vue";
import storageRef from "@/methods/storage";

defineExpose({
  addFilterLabel: (label: {key: string, value?: string}) => {
    const selector = `${label.key}:${label.value}`;
    if (filterValueInternal.value.includes(selector)) {
      return;
    }

    filterValueInternal.value += (filterValueInternal.value ? " " : "") + selector;
  }
});

const dots = "...";

const props = defineProps<{
  pagination?: boolean,
  search?: boolean,
  opts?: WatchOptions | WatchJoinOptions[] | object,
  sortOptions?: {id: string, desc: string, descending?: boolean}[],
  filterValue?: string
}>();

const itemsPerPage = [5, 10, 25, 50, 100]

const sortOptionsVariants = computed(() => {
  if (!props.sortOptions) {
    return [];
  }

  return props.sortOptions.map((opt) => {
    return opt.desc;
  });
});

const { opts, filterValue } = toRefs(props);

const items: Ref<Resource[]> = ref([]);

const optsList = props.opts as WatchJoinOptions[];

const filterValueInternal = ref("");
const currentPage = ref(1);
const selectedItemsPerPage: Ref<number> = storageRef(localStorage, 'itemsPerPage', 10);
const selectedSortOption: Ref<string | null> = ref(sortOptionsVariants?.value?.length ? sortOptionsVariants.value[0] : null);

const filterValueComputed = computed(() => {
  return filterValue.value !== undefined ? filterValue.value : filterValueInternal.value;
});

const offset = computed(() => {
  return (currentPage.value - 1) * selectedItemsPerPage.value;
});

const sortByState = computed(() => {
  if (!props.sortOptions) {
    return {};
  }

  for (const opt of props.sortOptions) {
    if (opt.desc === selectedSortOption?.value) {
      return {
        sortByField: opt.id,
        sortDescending: opt.descending
      };
    }
  }

  return {};
});

const watchOptions = computed<WatchOptions>(() => {
  const watchSingle = opts?.value;
  const watchJoin = opts?.value as WatchJoinOptions[];

  return (watchJoin?.length ? watchJoin[0] : watchSingle) as WatchOptions;
});

const paginationState = computed(() => {
  if (!props.pagination) {
    return {};
  }

  return {
    limit: selectedItemsPerPage.value,
    offset: offset.value,
  }
});

// reset the pagination when the search query changes
vueWatch(filterValue, () => {
  currentPage.value = 1;
})

const searchState = computed(() => {
  if (!props.search) {
    return {};
  }

  const o = watchOptions.value;

  if (!o) {
    return {};
  }

  // do not proceed if the pagination is not reset yet - when the currentPage is reset, this will get triggered again
  if (currentPage.value != 1) {
    return {}
  }

  const parts = filterValueComputed.value.split(" ");
  const selectors: string[] = [];
  const searchFor: string[] = [];

  for (const part of parts) {
    const match = part.match(/^(.+):(.*)$/);

    if (!match || match.length < 3) {
      if (part)
        searchFor.push(part);

      continue
    }

    selectors.push(`${match[1]}=${match[2]}`);
  }

  const res: {selectors?: string[], searchFor?: string[]} = {
    selectors: (o.selectors ?? []).concat(selectors)
  }

  if (searchFor.length > 0) {
    res.searchFor = searchFor;
  }

  return res;
});

const searchQuery = computed(() => {
  if (!searchState.value.searchFor) {
    return undefined;
  }

  return searchState.value.searchFor.join(" ");
});

const setupWatch = () => {
  const w = new Watch(items);

  w.setup(computed(() => {
    if (!opts?.value) {
      return;
    }

    return {
      ...paginationState.value,
      ...opts.value as WatchOptions,
      ...searchState.value,
      ...sortByState.value,
    }
  }));

  return w;
}

const setupJoinWatch = () => {
  const w = new WatchJoin(items);

  w.setup(computed(() => {
    if (!opts?.value) {
      return;
    }

    return {
      ...paginationState.value,
      ...(opts.value as WatchJoinOptions[])[0],
      ...searchState.value,
      ...sortByState.value,
    }
  }),
  computed(() => {
    if (!opts?.value) {
      return;
    }

    const o = opts.value as WatchJoinOptions[];

    return o.slice(1, o.length);
  }));

  return w;
}

const paginationRange = computed(() => {
  let ranges: number[][];
  if (totalPageCount.value < 20) {
    ranges = [
      [1, totalPageCount.value]
    ];
  } else {
    if (currentPage.value < 5 || currentPage.value > totalPageCount.value - 4) {
      ranges = [
        [1, 5],
        [totalPageCount.value - 4, totalPageCount.value]
      ]
    } else {
      ranges = [
        [1, 3],
        [currentPage.value - 1, currentPage.value + 1],
        [totalPageCount.value - 2,totalPageCount.value]
      ]
    }
  }

  const res: (string | number)[] = [];
  for (let i: number = 0; i < ranges.length; i++) {
    for (let j: number = ranges[i][0]; j <= ranges[i][1]; j++) {
      res.push(j);
    }

    if (i !== ranges.length - 1) {
      res.push(dots);
    }
  }

  return res;
});

const watch = optsList?.length ? setupJoinWatch() : setupWatch();
const err = watch.err;
const loading = watch.loading;
const itemsCount = watch.total;

const totalPageCount = computed(() => {
  return Math.ceil(watch.total.value / selectedItemsPerPage.value);
});

const showPageSelector = computed(() => {
  return props.pagination && totalPageCount.value > 1;
});

const prevPage = () => {
  currentPage.value = Math.max(1, currentPage.value - 1);
};

const nextPage = () => {
  currentPage.value = Math.min(totalPageCount.value, currentPage.value + 1);
};

const openPage = (page: number | string) => {
  if (page === dots) {
    return;
  }

  currentPage.value = page as number;
};
</script>

<style scoped>
.pagination-icon {
  @apply fill-current text-naturals-N8 cursor-pointer transition-all duration-200 hover:text-naturals-N10 w-5 h-5;
}
.pagination-icon-disabled {
  @apply text-naturals-N6;
}
.pagination-pages {
  @apply flex items-center transition-all duration-200 gap-2;
}
.pagination-page-number {
  @apply w-7 h-7 rounded text-naturals-N8 transition-all duration-200 flex items-center justify-center cursor-pointer hover:text-naturals-N9 select-none;
}
.unhovered {
  @apply hover:text-naturals-N8 cursor-default;
}
.pagination-page-number-active {
  @apply text-naturals-N12 bg-naturals-N4;
}

.loading-spinner {
  @apply absolute top-2/4 w-6 h-6;
}
</style>
