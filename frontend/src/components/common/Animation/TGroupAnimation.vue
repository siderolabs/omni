<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <transition-group>
    <slot />
  </transition-group>
</template>

<style>
.v-item {
  opacity: 1;
}
.v-enter-active,
.v-leave-active {
  transition: all 0.3s linear;
}
.v-enter-from,
.v-leave-to {
  opacity: 0;
}
</style>
