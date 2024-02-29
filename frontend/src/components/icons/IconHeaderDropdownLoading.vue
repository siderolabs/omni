<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <svg
    class="icon"
    viewBox="0 0 24 24"
    fill="currentColor"
    xmlns="http://www.w3.org/2000/svg"
    :active="active"
  >
    <path
      d="M7 12C7 13.1046 6.10457 14 5 14C3.89543 14 3 13.1046 3 12C3 10.8954 3.89543 10 5 10C6.10457 10 7 10.8954 7 12Z"
    />
    <path
      d="M14 12C14 13.1046 13.1046 14 12 14C10.8954 14 10 13.1046 10 12C10 10.8954 10.8954 10 12 10C13.1046 10 14 10.8954 14 12Z"
    />
    <path
      d="M21 12C21 13.1046 20.1046 14 19 14C17.8954 14 17 13.1046 17 12C17 10.8954 17.8954 10 19 10C20.1046 10 21 10.8954 21 12Z"
    />
  </svg>
</template>

<script setup lang="ts">
type Props = {
  active: boolean;
};

defineProps<Props>();
</script>

<style scoped>
.icon {
  @apply flex justify-center items-center fill-current transition-transform;
  width: 16px;
  height: 16px;
}

.icon[active=true] path:nth-child(1) {
  animation: loadingDots 2s ease-in-out infinite;
  animation-delay: 0s;
}
.icon[active=true] path:nth-child(2) {
  animation: loadingDots 2s ease-in-out infinite;
  animation-delay: 0.2s;
}
.icon[active=true] path:nth-child(3) {
  animation: loadingDots 2s ease-in-out infinite;
  animation-delay: 0.4s;
}
@keyframes loadingDots {
  0% {
    transform: translateY(0);
  }
  15% {
    transform: translateY(-5px);
  }
  35% {
    transform: translateY(0);
  }
}
</style>