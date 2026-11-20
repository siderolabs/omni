// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

import { containerError } from './containerError'

test.each<[ContainerStatusSpec, { summary: string; full: string } | undefined]>([
  [{}, undefined],
  [{ exitCode: 137 }, { summary: 'Exit 137', full: 'Exit 137' }],
  [
    {
      exitCode: 1,
      error:
        'task "crashloop-12" failed: exit code 1 (last log "starting\\nopen /var/log/journal: permission denied\\n")',
    },
    {
      summary: 'Exit 1: open /var/log/journal: permission denied',
      full: 'task "crashloop-12" failed: exit code 1\n\nLast log:\nstarting\nopen /var/log/journal: permission denied',
    },
  ],
  // colour codes in the log are dropped
  [
    {
      exitCode: 1,
      error: 'task "x" failed: exit code 1 (last log "\\x1b[31merror\\x1b[0m open /x: denied")',
    },
    {
      summary: 'Exit 1: error open /x: denied',
      full: 'task "x" failed: exit code 1\n\nLast log:\nerror open /x: denied',
    },
  ],
  // the process runner's form, without an exit code
  [
    { error: 'signal: killed (last log "out of memory")' },
    {
      summary: 'signal: killed: out of memory',
      full: 'signal: killed\n\nLast log:\nout of memory',
    },
  ],
  // an empty last log leaves the message alone
  [
    { exitCode: 2, error: 'task "x" failed: exit code 2 (last log "")' },
    { summary: 'task "x" failed: exit code 2', full: 'task "x" failed: exit code 2' },
  ],
  [
    { error: 'failed to create task runner: no such image' },
    {
      summary: 'failed to create task runner: no such image',
      full: 'failed to create task runner: no such image',
    },
  ],
  [
    { exitCode: 3, error: 'oops' },
    { summary: 'Exit 3: oops', full: 'Exit 3: oops' },
  ],
])('containerError(%j)', (status, expected) => {
  expect(containerError(status)).toEqual(expected)
})
