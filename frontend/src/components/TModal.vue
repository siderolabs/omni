<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="container-modal" v-if="view">
    <component :is="view" v-bind="props" />
  </div>
</template>

<script setup lang="ts">
import { watch, ref, type Component, shallowRef, type Ref } from "vue";
import { useRoute } from "vue-router";
import { modals } from "@/router";
import { modal } from "@/modal";

const route = useRoute();
const view: Ref<Component | null> = shallowRef(null);
const props = ref({});

const updateState = () => {
  if (modal.value) {
    view.value = modal.value.component;
    props.value = modal.value.props || {};

    return;
  }

  props.value = {};
  view.value = route.query.modal
    ? modals[route.query.modal as string]
    : null;
};

watch(
  () => route.query.modal,
  () => {
    if (route.query.modal) modal.value = null;

    updateState();
  }
);

// modals which do not need to be tied to the URI
watch(modal, () => {
  updateState();
});

updateState();
</script>

<style scoped>
.container-modal {
  @apply fixed left-0 right-0 bottom-0 top-0 z-30 flex justify-center items-center py-4;
  background-color: rgba(16, 17, 24, 0.9);
}
</style>
