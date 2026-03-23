// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'

import TInput from './TInput.vue'

test('is accessible with inline label', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
    },
  })

  await expect.element(screen.getByLabelText('My input')).toBeInTheDocument()
})

test('is accessible with overhead label', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
      overheadTitle: true,
    },
  })

  await expect.element(screen.getByLabelText('My input')).toBeInTheDocument()
})

test('allows input', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: 'hello',
      title: 'My input',
    },
  })

  await expect.element(screen.getByLabelText('My input')).toHaveValue('hello')

  await userEvent.type(screen.getByLabelText('My input'), 'potatoes')

  await expect.element(screen.getByLabelText('My input')).toHaveValue('hellopotatoes')
})

test('is clearable', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: 'hello',
      title: 'My input',
    },
  })

  await expect.element(screen.getByLabelText('My input')).toHaveValue('hello')

  await userEvent.click(screen.getByRole('button', { name: 'clear' }))

  await expect.element(screen.getByLabelText('My input')).toHaveValue('')
})

test('is focusable', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
    },
  })

  await expect.element(screen.getByLabelText('My input')).not.toHaveFocus()

  // trying to test for component being initially focused fails, rerender instead
  await screen.rerender({ focus: true })

  await expect.element(screen.getByLabelText('My input')).toHaveFocus()
})

test('is disableable', async () => {
  const screen = await render(TInput, {
    props: {
      modelValue: '',
      title: 'My input',
      disabled: true,
    },
  })

  await expect.element(screen.getByLabelText('My input')).toBeDisabled()

  await screen.rerender({ disabled: false })

  await expect.element(screen.getByLabelText('My input')).not.toBeDisabled()
})
