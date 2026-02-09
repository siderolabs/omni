<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  type AnchorHTMLAttributes,
  type ButtonHTMLAttributes,
  computed,
  type HTMLAttributes,
  useAttrs,
} from 'vue'
import type { RouterLinkProps } from 'vue-router'
import { RouterLink } from 'vue-router'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { cn } from '@/methods/utils'

interface Props {
  variant?: 'primary' | 'secondary' | 'subtle' | 'highlighted'
  size?: 'md' | 'sm' | 'xs' | 'xxs'
  icon?: IconType
  iconPosition?: 'left' | 'right'
  class?: HTMLAttributes['class']
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
  disabled?: boolean
}

interface RLink extends RouterLinkProps {
  // eslint-disable-next-line vue/no-required-prop-with-default
  is: 'router-link'
  href?: never
  to: RouterLinkProps['to']
  disabled?: boolean
}

const {
  is = 'button',
  to,
  href,
  variant = 'primary',
  size = 'md',
  iconPosition = 'right',
  icon,
  disabled,
  class: className,
} = defineProps<Props & (ButtonProps | AnchorProps | RLink)>()
const attrs = useAttrs()

const dynamicProps = computed(() => {
  // <a> does not support disabled, so we force <button> in that case
  if (!disabled) {
    if (to) return { to }
    if (href) return { href }
  }

  return { type: (attrs.type as string) || 'button', disabled }
})
</script>

<template>
  <component
    :is="disabled ? 'button' : is === 'router-link' ? RouterLink : is"
    v-bind="dynamicProps"
    class="flex items-center justify-center gap-1 rounded border transition-colors duration-200"
    :class="
      cn(
        {
          'border-naturals-n5 bg-naturals-n3 text-naturals-n12 hover:border-primary-p3 hover:bg-primary-p3 hover:text-naturals-n14 focus:border-primary-p2 focus:bg-primary-p2 focus:text-naturals-n14 active:border-primary-p4 active:bg-primary-p4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-naturals-n4 disabled:text-naturals-n7':
            variant === 'primary',
          'border-naturals-n5 bg-transparent text-naturals-n10 hover:bg-naturals-n5 hover:text-naturals-n14 focus:border-naturals-n7 focus:bg-naturals-n5 focus:text-naturals-n14 active:border-naturals-n5 active:bg-naturals-n4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-transparent disabled:text-naturals-n6':
            variant === 'secondary',
          'border-none bg-transparent text-naturals-n13 hover:text-primary-p3 focus:text-primary-p2 focus:underline active:text-primary-p4 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-n6':
            variant === 'subtle',
          'border-primary-p2 bg-primary-p4 text-naturals-n14 hover:border-primary-p3 hover:bg-primary-p3 hover:text-naturals-n14 focus:border-primary-p2 focus:bg-primary-p2 focus:text-naturals-n14 active:border-primary-p4 active:bg-primary-p4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-naturals-n4 disabled:text-naturals-n7':
            variant === 'highlighted',
          'px-4 py-1.5 text-sm': size === 'md',
          'px-2 py-0.5 text-sm': size === 'sm',
          'p-0 text-sm': size === 'xs',
          'p-0 text-xs': size === 'xxs',
        },
        className,
      )
    "
  >
    <span v-if="$slots.default" class="whitespace-nowrap">
      <slot />
    </span>
    <TIcon
      v-if="icon"
      :icon="icon"
      :class="[
        size === 'xxs' ? 'size-3' : 'size-4',
        {
          'order-first': iconPosition === 'left',
        },
      ]"
    />
  </component>
</template>
