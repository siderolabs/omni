// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useTimeout } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import {
  type AsyncComponentLoader,
  type Component,
  defineAsyncComponent,
  defineComponent,
  h,
  type StyleValue,
} from 'vue'

import TSpinner from '@/components/Spinner/TSpinner.vue'
import { cn } from '@/methods/utils'

interface LazyComponentOptions {
  /** Applied to the placeholder over the parent's style, so it looks like the loaded component's background. */
  placeholderStyle?: StyleValue
}

/**
 * Wraps a heavy component so its code is only downloaded when it is first rendered.
 *
 * Until then a placeholder takes its place. Vue renders it with the async component's attrs, so the class the parent
 * gives the lazy component (e.g. `size-full`) sizes it the same way and reserves the space straight away. A centred
 * spinner only appears once loading takes noticeably long, so fast loads don't flash it.
 */
export function defineLazyComponent<T extends Component>(
  loader: AsyncComponentLoader<T>,
  { placeholderStyle }: LazyComponentOptions = {},
) {
  const LazyComponentPlaceholder = defineComponent({
    inheritAttrs: false,
    setup(_, { attrs }) {
      const showSpinner = useTimeout(200)

      // Only class and style are forwarded; the other attrs are the real component's props.
      return () =>
        h(
          'div',
          {
            class: cn('flex items-center justify-center', attrs.class as ClassValue),
            style: [attrs.style as StyleValue, placeholderStyle],
          },
          showSpinner.value ? h(TSpinner, { class: 'size-6' }) : undefined,
        )
    },
  })

  return defineAsyncComponent({
    loader,
    loadingComponent: LazyComponentPlaceholder,
    // using useTimeout instead of this delay so the root div loads immediately
    // Only the spinner needs to be delayed, to prevent CLS
    delay: 0,
  })
}
