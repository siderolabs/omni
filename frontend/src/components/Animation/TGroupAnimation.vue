<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <TransitionGroup>
    <slot />
  </TransitionGroup>
</template>

<!-- eslint-disable-next-line vue/enforce-style-attribute -->
<style>
.v-enter-active,
.v-leave-active {
  transition: all 0.3s linear;
}
.v-enter-from,
.v-leave-to {
  opacity: 0;
}
</style>
