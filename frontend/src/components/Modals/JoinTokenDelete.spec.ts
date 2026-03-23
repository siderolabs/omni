// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'
import { defineComponent, onMounted } from 'vue'

import { deleteJoinToken } from '@/methods/auth'
import { showError } from '@/notification'

import JoinTokenDelete from './JoinTokenDelete.vue'

vi.mock('@/methods/auth', () => ({
  deleteJoinToken: vi.fn(),
}))

vi.mock('@/notification', () => ({
  showError: vi.fn(),
}))

const ReadyWarningsStub = defineComponent({
  emits: ['ready'],
  setup(_, { emit }) {
    onMounted(() => emit('ready'))
    return () => null
  },
})

const PendingWarningsStub = defineComponent({
  emits: ['ready'],
  setup() {
    return () => null
  },
})

const TOKEN = 'test-token-abc123'

describe('JoinTokenDelete', () => {
  beforeEach(() => vi.clearAllMocks())

  test('renders the token in the title', async () => {
    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByText(`Delete the token ${TOKEN} ?`)).toBeInTheDocument())
  })

  test('shows the permanent deletion warning', async () => {
    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() =>
      expect(screen.getByText(/This will permanently delete the Join Token/)).toBeInTheDocument(),
    )
  })

  test('action button is disabled before warnings are ready', async () => {
    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: PendingWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled())
  })

  test('action button is enabled once warnings are ready', async () => {
    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled())
  })

  test('calls deleteJoinToken with the token when confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteJoinToken).mockResolvedValue(undefined)

    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    expect(deleteJoinToken).toHaveBeenCalledExactlyOnceWith(TOKEN)
  })

  test('closes the modal after a successful delete', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteJoinToken).mockResolvedValue(undefined)
    const onUpdateOpen = vi.fn()

    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true, 'onUpdate:open': onUpdateOpen },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(onUpdateOpen).toHaveBeenCalledWith(false))
    expect(showError).not.toHaveBeenCalled()
  })

  test('shows an error and still closes the modal when delete fails', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteJoinToken).mockRejectedValue(new Error('Network error'))
    const onUpdateOpen = vi.fn()

    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true, 'onUpdate:open': onUpdateOpen },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      expect(showError).toHaveBeenCalledExactlyOnceWith('Failed to Delete Token', 'Network error')
      expect(onUpdateOpen).toHaveBeenCalledWith(false)
    })
  })

  test('closes the modal when Cancel is clicked without deleting', async () => {
    const user = userEvent.setup()

    const screen = render(JoinTokenDelete, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => screen.getByRole('button', { name: 'Cancel' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(deleteJoinToken).not.toHaveBeenCalled()
  })
})
