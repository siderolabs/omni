// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { ComponentProps } from 'vue-component-type-helpers'

import StatusPill from './StatusPill.vue'

type Props = ComponentProps<typeof StatusPill>

const tones: Props['tone'][] = ['success', 'warning', 'danger', 'info']
const glyphs: Props['glyph'][] = [
  'success',
  'danger',
  'warning',
  'progress',
  'neutral',
  'unknown',
  'dot',
]

const meta: Meta<typeof StatusPill> = {
  component: StatusPill,
  argTypes: {
    tone: {
      control: 'inline-radio',
      options: tones,
    },
    glyph: {
      control: 'select',
      options: glyphs,
    },
    default: {
      control: 'text',
    },
  },
  args: {
    tone: 'success',
    default: 'Running',
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const AllPills: Story = {
  decorators: [
    // One column per glyph, after the column for the glyph the tone implies
    () => ({ template: '<div class="grid grid-cols-8 items-center gap-tight"><story/></div>' }),
  ],
  render: () => ({
    components: { StatusPill },
    template: tones
      .flatMap((tone) => [
        `<StatusPill tone="${tone}">${tone}</StatusPill>`,
        ...glyphs.map(
          (glyph) => `<StatusPill tone="${tone}" glyph="${glyph}">${tone}-${glyph}</StatusPill>`,
        ),
      ])
      .join(''),
  }),
}
