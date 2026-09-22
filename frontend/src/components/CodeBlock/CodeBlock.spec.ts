// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen, waitFor } from '@testing-library/vue'
import { expect, test } from 'vitest'

import CodeBlock from './CodeBlock.vue'

test('highlights the code it is given', async () => {
  const { container } = render(CodeBlock, { props: { code: 'echo hello', lang: 'shellscript' } })

  // Plain text first, so the block is readable before the grammar loads.
  expect(screen.getByText('echo hello')).toBeInTheDocument()

  await waitFor(() => expect(container.querySelectorAll('pre span').length).toBeGreaterThan(1))
  expect(container.querySelector('pre')).toHaveTextContent('echo hello')
})

test('marks search matches without losing syntax colours', async () => {
  const { container } = render(CodeBlock, {
    props: { code: '{\n  "name": "talos-node-1"\n}', lang: 'json', search: 'NAME' },
  })

  await waitFor(() => expect(container.querySelectorAll('.code-search-match')).toHaveLength(1))

  const [match] = container.querySelectorAll<HTMLElement>('.code-search-match')
  expect(match).toHaveTextContent('name')
  // The match keeps the token colour it had before being decorated.
  expect(match).toHaveAttribute('style', expect.stringContaining('color'))
})
