// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, onMounted } from 'vue'

import { revokeJoinToken } from '@/methods/auth'
import { showError } from '@/notification'

import JoinTokenRevoke from './JoinTokenRevoke.vue'

vi.mock('@/methods/auth', () => ({
  revokeJoinToken: vi.fn(),
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

const TOKEN = 'test-token-xyz789'

describe('JoinTokenRevoke', () => {
  beforeEach(() => vi.clearAllMocks())

  test('renders the token in the title', async () => {
    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByText(`Revoke the token ${TOKEN} ?`)).toBeInTheDocument())
  })

  test('shows the confirmation message', async () => {
    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByText('Please confirm the action.')).toBeInTheDocument())
  })

  test('action button is disabled before warnings are ready', async () => {
    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: PendingWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke' })).toBeDisabled())
  })

  test('action button is enabled once warnings are ready', async () => {
    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke' })).toBeEnabled())
  })

  test('calls revokeJoinToken with the token when confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(revokeJoinToken).mockResolvedValue(undefined)

    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Revoke' }))

    expect(revokeJoinToken).toHaveBeenCalledExactlyOnceWith(TOKEN)
  })

  test('closes the modal after a successful revoke', async () => {
    const user = userEvent.setup()
    vi.mocked(revokeJoinToken).mockResolvedValue(undefined)
    const onUpdateOpen = vi.fn()

    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true, 'onUpdate:open': onUpdateOpen },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Revoke' }))

    await waitFor(() => expect(onUpdateOpen).toHaveBeenCalledWith(false))
    expect(showError).not.toHaveBeenCalled()
  })

  test('shows an error and still closes the modal when revoke fails', async () => {
    const user = userEvent.setup()
    vi.mocked(revokeJoinToken).mockRejectedValue(new Error('Permission denied'))
    const onUpdateOpen = vi.fn()

    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true, 'onUpdate:open': onUpdateOpen },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Revoke' }))

    await waitFor(() => {
      expect(showError).toHaveBeenCalledExactlyOnceWith(
        'Failed to Revoke Token',
        'Permission denied',
      )
      expect(onUpdateOpen).toHaveBeenCalledWith(false)
    })
  })

  test('closes the modal when Cancel is clicked without revoking', async () => {
    const user = userEvent.setup()

    render(JoinTokenRevoke, {
      props: { token: TOKEN, open: true },
      global: { stubs: { JoinTokenWarnings: ReadyWarningsStub } },
    })

    await waitFor(() => screen.getByRole('button', { name: 'Cancel' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(revokeJoinToken).not.toHaveBeenCalled()
  })
})
