// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
/** Removes ANSI colour codes (SGR sequences such as `\x1b[31m`) that programs write into their logs. */
export function stripAnsiColours(text: string) {
  return text.replace(/\x1b\[[\d;]*m/g, '')
}
