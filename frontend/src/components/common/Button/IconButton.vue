<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { type AnchorHTMLAttributes, type ButtonHTMLAttributes, computed, useAttrs } from 'vue'
import type { RouterLinkProps } from 'vue-router'
import { RouterLink } from 'vue-router'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

interface Props {
  icon?: IconType
  danger?: boolean
  iconClasses?: unknown
}

interface ButtonProps extends /* @vue-ignore */ ButtonHTMLAttributes {
  is?: 'button'
  href?: never
  to?: never
}

interface AnchorProps extends /* @vue-ignore */ AnchorHTMLAttributes {
  // eslint-disable-next-line vue/no-required-prop-with-default
  is: 'a'
  href: string
  to?: never
}

interface RLink extends RouterLinkProps {
  // eslint-disable-next-line vue/no-required-prop-with-default
  is: 'router-link'
  href?: never
  to: RouterLinkProps['to']
}

const { is = 'button', to, href } = defineProps<Props & (ButtonProps | AnchorProps | RLink)>()
const attrs = useAttrs()

const dynamicProps = computed(() => {
  if (to) return { to }
  if (href) return { href }
  return { type: (attrs.type as string) || 'button' }
})
</script>

<template>
  <component
    :is="is === 'router-link' ? RouterLink : is"
    v-bind="dynamicProps"
    class="h-6 w-6 rounded align-top text-naturals-n11 transition-all duration-100 hover:bg-naturals-n7 hover:text-naturals-n13 disabled:pointer-events-none disabled:cursor-not-allowed disabled:text-naturals-n7"
    :class="{ 'text-red-r1': danger }"
  >
    <slot>
      <TIcon class="h-full w-full cursor-pointer p-1" :class="iconClasses" :icon="icon" />
    </slot>
  </component>
</template>
