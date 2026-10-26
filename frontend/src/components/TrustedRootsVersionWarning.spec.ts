// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { server } from '@msw/server'
import { render, screen, waitFor } from '@testing-library/vue'
import { flushPromises } from '@vue/test-utils'
import { http, HttpResponse } from 'msw'
import { describe, expect, test } from 'vitest'

import TrustedRootsVersionWarning from './TrustedRootsVersionWarning.vue'

const title = "This Talos version cannot carry Omni's custom CA"

// serve answers the SideroLink API config read with the given trusted roots and counts the reads.
function serve(trustedRoots: string) {
  const reads = { count: 0 }

  server.use(
    http.post('/omni.resources.ResourceService/Get', async ({ request }) => {
      const { type, id } = (await request.json()) as { type: string; id: string }

      reads.count++

      return HttpResponse.json({
        body: JSON.stringify({ metadata: { id, type }, spec: { trusted_roots: trustedRoots } }),
      })
    }),
  )

  return reads
}

// settle waits until the config read is answered and rendered.
async function settle(reads: { count: number }) {
  await waitFor(() => expect(reads.count).toBe(1))
  await flushPromises()
}

describe('TrustedRootsVersionWarning', () => {
  test('warns below Talos 1.12 when Omni has trusted roots, without leading whitespace', async () => {
    serve('-----BEGIN CERTIFICATE-----')

    const { container } = render(TrustedRootsVersionWarning, { props: { talosVersion: '1.11.5' } })

    await waitFor(() => expect(screen.getByText(title)).toBeInTheDocument())

    expect(container.querySelector('p')?.textContent).toMatch(/^Talos below 1\.12/)
  })

  test('stays hidden from Talos 1.12 on and warns for its prereleases, as Omni treats them', async () => {
    const reads = serve('-----BEGIN CERTIFICATE-----')

    const { rerender } = render(TrustedRootsVersionWarning, { props: { talosVersion: '1.12.0' } })

    await settle(reads)

    expect(screen.queryByText(title)).not.toBeInTheDocument()

    await rerender({ talosVersion: '1.12.0-alpha.1' })

    expect(screen.getByText(title)).toBeInTheDocument()
  })

  test('stays hidden when Omni has no trusted roots', async () => {
    const reads = serve('')

    render(TrustedRootsVersionWarning, { props: { talosVersion: '1.11.5' } })

    await settle(reads)

    expect(screen.queryByText(title)).not.toBeInTheDocument()
  })

  test('stays hidden without a version or with one that does not parse', async () => {
    const reads = serve('-----BEGIN CERTIFICATE-----')

    const { rerender } = render(TrustedRootsVersionWarning)

    await settle(reads)

    expect(screen.queryByText(title)).not.toBeInTheDocument()

    await rerender({ talosVersion: 'latest' })

    expect(screen.queryByText(title)).not.toBeInTheDocument()
  })
})
