// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Phases a volume reaches once it is prepared, which is when encryption is set up.
const preparedPhases = new Set(['prepared', 'ready', 'closed'])

/** Whether the volume got as far as being prepared, so its encryption is known. */
export function isVolumePrepared(phase?: string): boolean {
  return preparedPhases.has(phase ?? '')
}
