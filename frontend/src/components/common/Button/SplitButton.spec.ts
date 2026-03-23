// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'

import SplitButton from './SplitButton.vue'

test('sends click events', async () => {
  const clickFn = vi.fn()

  const screen = await render(SplitButton, {
    props: {
      actions: ['one', 'two', 'three'],
      onClick: clickFn,
    },
  })

  await userEvent.click(screen.getByRole('button', { name: 'one' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('one')

  clickFn.mockClear()

  await userEvent.click(screen.getByRole('button', { name: 'extra actions' }))
  await userEvent.click(screen.getByRole('menuitem', { name: 'one' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('one')

  clickFn.mockClear()

  await userEvent.click(screen.getByRole('button', { name: 'extra actions' }))
  await userEvent.click(screen.getByRole('menuitem', { name: 'two' }))
  expect(clickFn).toHaveBeenCalledExactlyOnceWith('two')
})
