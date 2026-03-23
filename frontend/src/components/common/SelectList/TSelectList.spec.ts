// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'

import TSelectList from './TSelectList.vue'

enableAutoUnmount(afterEach)

test('is accessible with inline label', async () => {
  const screen = await render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
    },
  })

  await await expect.element(screen.getByLabelText('My select')).toBeInTheDocument()
})

test('is accessible with overhead label', async () => {
  const screen = await render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      overheadTitle: true,
    },
  })

  await expect.element(screen.getByLabelText('My select')).toBeInTheDocument()
})

test('accepts a default value', async () => {
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

  const screen = await render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      defaultValue: 'first option',
      'onUpdate:modelValue': updateFn,
      onCheckedValue: checkedFn,
    },
  })

  const trigger = screen.getByLabelText('My select')

  expect(updateFn).toHaveBeenCalledExactlyOnceWith('first option')
  expect(checkedFn).not.toHaveBeenCalled()

  await expect.element(trigger).toHaveTextContent('first option')

  // Open dropdown
  await userEvent.click(trigger)

  await expect
    .element(screen.getByRole('option', { name: 'first option' }))
    .toHaveAttribute('aria-selected', 'true')
})

test.skip('allows selection', async () => {
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

  const screen = await render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      'onUpdate:modelValue': updateFn,
      onCheckedValue: checkedFn,
    },
  })

  const trigger = screen.getByLabelText('My select')

  expect.element(trigger).toHaveTextContent('My select')

  // Open dropdown
  await userEvent.click(trigger)

  const option = screen.getByRole('option', { name: 'second option' })

  // Select option
  await userEvent.click(option)

  expect(updateFn).toHaveBeenCalledExactlyOnceWith('second option')
  expect(checkedFn).toHaveBeenCalledExactlyOnceWith('second option')

  await expect.element(trigger).toHaveTextContent('second option')
  await expect.element(option).toHaveAttribute('aria-selected', 'true')
})

test.skip('exposes selectItem', async () => {
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

  // FIXME: maybe now is possibru
  // Can't test defineExpose with testing-library, using @vue/test-utils instead
  const wrapper = mount(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      'onUpdate:modelValue': updateFn,
      onCheckedValue: checkedFn,
    },
  })

  expect(wrapper.text()).not.toContain('second option')

  wrapper.vm.selectItem('second option')

  expect(updateFn).toHaveBeenCalledExactlyOnceWith('second option')
  expect(checkedFn).toHaveBeenCalledExactlyOnceWith('second option')

  await expect.element(wrapper.text()).toContain('second option')
})

test('focuses search on open', async () => {
  const screen = await render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      searcheable: true,
    },
  })

  const trigger = screen.getByLabelText('My select')

  // Open dropdown
  await userEvent.click(trigger)

  await expect.element(screen.getByRole('textbox', { name: 'search' })).toHaveFocus()
})
