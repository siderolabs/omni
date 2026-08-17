// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import {
  formatDate,
  formatDistanceToNowStrict,
  formatRFC7231,
  fromUnixTime,
  parseISO,
} from 'date-fns'

export function relativeISO(input: string) {
  return formatDistanceToNowStrict(parseISO(input), { addSuffix: true })
}

export function formatISO(input: string, format = 'dd/MM/yyyy HH:mm:ss') {
  return formatDate(parseISO(input), format)
}

export function formatFullDateTime(time?: string) {
  if (!time) return 'Never'

  const date = /^\d+$/.test(time) ? fromUnixTime(parseInt(time)) : parseISO(time)

  return formatRFC7231(date)
}
