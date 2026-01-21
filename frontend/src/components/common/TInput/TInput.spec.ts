// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { expect, test } from 'vitest'

import TInput from './TInput.vue'

test('is accessible with inline label', () => {
  render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
    },
  })

  expect(screen.getByLabelText('My input')).toBeInTheDocument()
})

test('is accessible with overhead label', () => {
  render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
      overheadTitle: true,
    },
  })

  expect(screen.getByLabelText('My input')).toBeInTheDocument()
})

test('allows input', async () => {
  const user = userEvent.setup()

  render(TInput, {
    props: {
      modelValue: 'hello',
      title: 'My input',
    },
  })

  expect(screen.getByLabelText('My input')).toHaveValue('hello')

  await user.type(screen.getByLabelText('My input'), 'potatoes')

  expect(screen.getByLabelText('My input')).toHaveValue('hellopotatoes')
})

test('is clearable', async () => {
  const user = userEvent.setup()

  render(TInput, {
    props: {
      modelValue: 'hello',
      title: 'My input',
    },
  })

  expect(screen.getByLabelText('My input')).toHaveValue('hello')

  await user.click(screen.getByRole('button', { name: 'clear' }))

  expect(screen.getByLabelText('My input')).toHaveValue('')
})

test('is focusable', async () => {
  const { rerender } = render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
    },
  })

  expect(screen.getByLabelText('My input')).not.toHaveFocus()

  // due to jsdom limitations trying to test for component being initially focused fails
  await rerender({ focus: true })

  expect(screen.getByLabelText('My input')).toHaveFocus()
})

test('is disableable', async () => {
  const { rerender } = render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
      disabled: true,
    },
  })

  expect(screen.getByLabelText('My input')).toBeDisabled()

  await rerender({ disabled: false })

  expect(screen.getByLabelText('My input')).not.toBeDisabled()
})
