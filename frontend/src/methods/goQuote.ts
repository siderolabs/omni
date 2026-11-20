// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
const escapes: Record<string, string> = {
  a: '\x07',
  b: '\b',
  f: '\f',
  n: '\n',
  r: '\r',
  t: '\t',
  v: '\v',
}

/**
 * Decodes a string Go quoted with %q (strconv.Quote), quotes included. Not JSON.parse: Go also
 * writes \a, \v, \xNN and \UNNNNNNNN, e.g. \x1b for the ANSI colour codes common in logs.
 */
export function unquoteGo(quoted: string) {
  return quoted
    .slice(1, -1)
    .replace(/\\(x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8}|.)/g, (_, e: string) => {
      if (/^[xuU]/.test(e)) return String.fromCodePoint(parseInt(e.slice(1), 16))

      return escapes[e] ?? e
    })
}
