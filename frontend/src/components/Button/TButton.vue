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

import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
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
          'border-border-strong bg-surface-raised text-content-default hover:border-accent-fill hover:bg-accent-fill hover:text-content-on-accent focus:border-accent-fill focus:bg-accent-fill-hover focus:text-content-on-accent active:border-accent-fill-active active:bg-accent-fill-active active:text-content-on-accent disabled:cursor-not-allowed disabled:border-border-default disabled:bg-surface-hover disabled:text-content-disabled':
            variant === 'primary',
          'border-border-strong bg-transparent text-content-secondary hover:bg-surface-hover hover:text-content-emphasis focus:border-border-strong focus:bg-surface-hover focus:text-content-emphasis active:border-border-strong active:bg-surface-inert active:text-content-emphasis disabled:cursor-not-allowed disabled:border-border-default disabled:bg-transparent disabled:text-content-disabled':
            variant === 'secondary',
          'border-none bg-transparent text-content-emphasis hover:text-accent-text focus:text-accent-hover focus:underline active:text-accent-text active:no-underline disabled:cursor-not-allowed disabled:text-content-disabled':
            variant === 'subtle',
          'border-accent-fill bg-accent-fill text-content-on-accent hover:border-accent-fill-hover hover:bg-accent-fill-hover focus:border-accent-fill-hover focus:bg-accent-fill-hover active:border-accent-fill-active active:bg-accent-fill-active disabled:cursor-not-allowed disabled:border-border-default disabled:bg-surface-hover disabled:text-content-disabled':
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
    <span v-if="$slots.default" class="truncate">
      <slot />
    </span>
    <TIcon
      v-if="icon"
      :icon="icon"
      class="shrink-0"
      :class="[
        size === 'xxs' ? 'size-3' : 'size-4',
        {
          'order-first': iconPosition === 'left',
        },
      ]"
      aria-hidden="true"
    />
  </component>
</template>
