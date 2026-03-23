// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'

import RadioGroup from './RadioGroup.vue'
import RadioGroupOption from './RadioGroupOption.vue'

const options = Array(5)
  .fill(null)
  .map((_, i) => ({
    label: `option-${i}-label`,
    description: `option-${i}-desc`,
    value: `option-${i}-value`,
  }))

test('allows selection', async () => {
  const screen = await render(RadioGroup, {
    props: {
      label: 'My radio',
    },
    global: {
      components: { RadioGroupOption },
    },
    slots: {
      default: options
        .map(
          (option) => `
          <RadioGroupOption value="${option.value}">
            ${option.label}

            <template #description>
              ${option.description}
            </template>
          </RadioGroupOption>
        `,
        )
        .join(''),
    },
  })

  const radio = screen.getByRole('radio', { name: options[0].label })

  await expect.element(radio).not.toBeChecked()
  await userEvent.click(radio)
  await expect.element(radio).toBeChecked()
})
