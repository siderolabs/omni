<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="size-6">
    <mask id="toggle">
      <rect id="cover" x="-5%" y="-5%" width="110%" height="110%" fill="white" />
      <circle cx="18" cy="3" r="5" fill="black" />
    </mask>
    <path
      mask="url(#toggle)"
      stroke-linecap="round"
      stroke-width="1.5"
      stroke="currentColor"
      stroke-linejoin="round"
      d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z"
    />
    <circle cx="18" cy="3" r="3" fill="#FF5F2A" />
  </svg>
</template>
