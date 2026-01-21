// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { expect, test, vi } from 'vitest'

import SplitButton from './SplitButton.vue'

test('sends click events', async () => {
  const user = userEvent.setup()
  const clickFn = vi.fn()

  render(SplitButton, {
    props: {
      actions: ['one', 'two', 'three'],
      onClick: clickFn,
    },
  })

  await user.click(screen.getByRole('button', { name: 'one' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('one')

  clickFn.mockClear()

  await user.click(screen.getByRole('button', { name: 'extra actions' }))
  await user.click(screen.getByRole('menuitem', { name: 'one' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('one')

  clickFn.mockClear()

  await user.click(screen.getByRole('button', { name: 'extra actions' }))
  await user.click(screen.getByRole('menuitem', { name: 'two' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('two')
})
