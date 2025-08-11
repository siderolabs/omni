<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { type Component, type Ref, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'

import { modal } from '@/modal'
import { modals } from '@/router'

const route = useRoute()
const view: Ref<Component | null> = shallowRef(null)
const props = ref({})

const updateState = () => {
  if (modal.value) {
    view.value = modal.value.component
    props.value = modal.value.props || {}

    return
  }

  props.value = {}
  view.value = route.query.modal ? modals[route.query.modal as string] : null
}

watch(
  () => route.query.modal,
  () => {
    if (route.query.modal) modal.value = null

    updateState()
  },
)

// modals which do not need to be tied to the URI
watch(modal, () => {
  updateState()
})

updateState()
</script>

<template>
  <div
    v-if="view"
    class="fixed top-0 right-0 bottom-0 left-0 z-30 flex items-center justify-center bg-naturals-n0/90 py-4"
  >
    <component :is="view" v-bind="props" />
  </div>
</template>
