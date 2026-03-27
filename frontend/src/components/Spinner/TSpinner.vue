<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <svg class="spinner size-full" viewBox="0 0 70 70" xmlns="http://www.w3.org/2000/svg">
      <circle
        class="path stroke-primary-p3"
        fill="none"
        stroke-width="9"
        stroke-linecap="round"
        cx="35"
        cy="35"
        r="30"
      ></circle>
    </svg>
  </div>
</template>

<style scoped>
.spinner {
  animation: rotator 1.4s linear infinite;
}
@keyframes rotator {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(270deg);
  }
}
.path {
  stroke-dasharray: 187;
  stroke-dashoffset: 0;
  transform-origin: center;
  animation: dash 1.4s ease-in-out infinite;
}

@keyframes dash {
  0% {
    stroke-dashoffset: 187;
  }
  50% {
    stroke-dashoffset: 46.75;
    transform: rotate(135deg);
  }
  100% {
    stroke-dashoffset: 187;
    transform: rotate(450deg);
  }
}
</style>
