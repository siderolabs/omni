// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'
import { stripAnsiColours } from '@/methods/ansi'
import { unquoteGo } from '@/methods/goQuote'

// Talos ends an error with the process's last log line(s): `... (last log "starting\nopen /x: denied")`
const lastLogPattern = /^([\s\S]*) \(last log ("(?:[^"\\]|\\.)*")\)$/

export interface ContainerError {
  /** One line for the table: the exit code and the last thing the process logged, as the design has it. */
  summary: string
  /** Everything Talos said, with the last log decoded onto its own lines. */
  full: string
}

/**
 * What went wrong with a container. A task that exits non-zero fails with
 * `task "x" failed: exit code 1 (last log "...")`, so its summary is "Exit 1: <last log line>".
 * Anything else, such as a container that never started, is shown as Talos says it.
 */
export function containerError({
  error,
  exitCode,
}: ContainerStatusSpec): ContainerError | undefined {
  const exit = exitCode ? `Exit ${exitCode}` : undefined

  if (!error) return exit ? { summary: exit, full: exit } : undefined

  const match = lastLogPattern.exec(error)

  if (match) {
    const [, message, quoted] = match
    const lastLog = stripAnsiColours(unquoteGo(quoted))
    const lastLine = lastLog
      .split('\n')
      .map((l) => l.trim())
      .findLast(Boolean)

    if (lastLine) {
      return {
        summary: `${exit ?? message}: ${lastLine}`,
        full: `${message}\n\nLast log:\n${lastLog.trimEnd()}`,
      }
    }

    error = message
  }

  // The exit code is only worth adding when Talos didn't already say it
  const summary = exit && !error.includes(`exit code ${exitCode}`) ? `${exit}: ${error}` : error

  return { summary, full: summary }
}
