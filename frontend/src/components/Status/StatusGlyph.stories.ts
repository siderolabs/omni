// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { ComponentProps } from 'vue-component-type-helpers'

import StatusGlyph from './StatusGlyph.vue'

type Props = ComponentProps<typeof StatusGlyph>

const glyphs: Props['glyph'][] = ['success', 'danger', 'warning', 'progress', 'neutral', 'unknown']

const meta: Meta<typeof StatusGlyph> = {
  component: StatusGlyph,
  argTypes: {
    glyph: {
      control: 'select',
      options: glyphs,
    },
  },
  args: {
    class: 'size-8',
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const AllGlyphs: Story = {
  decorators: [
    () => ({ template: '<div class="grid grid-cols-6 items-center gap-tight"><story/></div>' }),
  ],
  render: () => ({
    components: { StatusGlyph },
    template: glyphs
      .map(
        (glyph) =>
          `<StatusGlyph tone="${glyph}" glyph="${glyph}" class="size-8">${glyph}-${glyph}</StatusGlyph>`,
      )
      .join(''),
  }),
}
