// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, it } from 'vitest'

import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'

import { diffMatches } from './matchUtils'

function match(cve: string, pkg: string): Match {
  return {
    vulnerability: {
      id: cve,
      severity: 'High',
      cvss: [],
    },
    relatedVulnerabilities: [],
    artifact: { name: pkg, version: '1.0.0' },
  } as unknown as Match
}

describe('diffMatches', () => {
  it('partitions findings into resolved, remaining and introduced', () => {
    const current = [match('CVE-1', 'a'), match('CVE-2', 'b'), match('CVE-3', 'c')]
    const target = [match('CVE-2', 'b'), match('CVE-4', 'd')]

    const diff = diffMatches(current, target)

    expect(diff.resolved.map((m) => m.vulnerability.id)).toEqual(['CVE-1', 'CVE-3'])
    expect(diff.remaining.map((m) => m.vulnerability.id)).toEqual(['CVE-2'])
    expect(diff.introduced.map((m) => m.vulnerability.id)).toEqual(['CVE-4'])
  })

  it('treats the same CVE in different packages as distinct findings', () => {
    const current = [match('CVE-1', 'a')]
    const target = [match('CVE-1', 'b')]

    const diff = diffMatches(current, target)

    expect(diff.resolved).toHaveLength(1)
    expect(diff.introduced).toHaveLength(1)
    expect(diff.remaining).toHaveLength(0)
  })
})
