// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { expect, test } from 'vitest'

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
  const user = userEvent.setup()

  render(RadioGroup, {
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

  await waitFor(() => {
    expect(screen.getByRole('radio', { name: options[0].label })).not.toBeChecked()
  })

  await user.click(screen.getByRole('radio', { name: options[0].label }))

  expect(screen.getByRole('radio', { name: options[0].label })).toBeChecked()
})
