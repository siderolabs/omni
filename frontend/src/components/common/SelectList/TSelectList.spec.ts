// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'

import TSelectList from './TSelectList.vue'

// Used by reka-ui select, test fails without it
window.HTMLElement.prototype.hasPointerCapture = vi.fn()

enableAutoUnmount(afterEach)

test('is accessible with inline label', () => {
  render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
    },
  })

  expect(screen.getByLabelText('My select')).toBeInTheDocument()
})

test('is accessible with overhead label', () => {
  render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      overheadTitle: true,
    },
  })

  expect(screen.getByLabelText('My select')).toBeInTheDocument()
})

test('accepts a default value', async () => {
  const user = userEvent.setup()
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

  render(TSelectList, {
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

  expect(trigger).toHaveTextContent('first option')

  // Open dropdown
  await user.click(trigger)

  expect(screen.getByRole('option', { name: 'first option' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
})

test('allows selection', async () => {
  const user = userEvent.setup()
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

  render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      'onUpdate:modelValue': updateFn,
      onCheckedValue: checkedFn,
    },
  })

  const trigger = screen.getByLabelText('My select')

  expect(trigger.textContent).toBe('My select') // Exact match to assert no default

  // Open dropdown
  await user.click(trigger)

  const option = screen.getByRole('option', { name: 'second option' })

  // Select option
  await user.click(option)

  expect(updateFn).toHaveBeenCalledExactlyOnceWith('second option')
  expect(checkedFn).toHaveBeenCalledExactlyOnceWith('second option')

  expect(trigger).toHaveTextContent('second option')
  expect(option).toHaveAttribute('aria-selected', 'true')
})

test('exposes selectItem', async () => {
  const updateFn = vi.fn()
  const checkedFn = vi.fn()

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

  await waitFor(() => {
    expect(wrapper.text()).toContain('second option')
  })
})

test('focuses search on open', async () => {
  const user = userEvent.setup()

  render(TSelectList, {
    props: {
      title: 'My select',
      values: ['first option', 'second option'],
      searcheable: true,
    },
  })

  const trigger = screen.getByLabelText('My select')

  // Open dropdown
  await user.click(trigger)

  expect(screen.getByRole('textbox', { name: 'search' })).toHaveFocus()
})
