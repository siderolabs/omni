// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, it } from 'vitest'

import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'
import report from '@/views/InstallationMedia/vulnerabilities/sample-report.json'

import { diffMatches, getVexNotes } from './matchUtils'

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

describe('getVexNotes', () => {
  function withStatements(...statements: Record<string, string>[]): Match {
    return {
      ...match('CVE-1', 'a'),
      matchDetails: [
        { matcher: 'stock-matcher', found: { vulnerabilityID: 'CVE-1' } },
        ...statements.map((Statement) => ({ matcher: 'openvex-matcher', found: { Statement } })),
      ],
    } as unknown as Match
  }

  it('extracts and humanises openvex statements', () => {
    const m = withStatements({
      status: 'not_affected',
      justification: 'vulnerable_code_not_present',
      impact_statement: 'Talos does not ship this code',
    })

    expect(getVexNotes(m)).toEqual([
      {
        status: 'not affected',
        justification: 'vulnerable code not present',
        impact: 'Talos does not ship this code',
        action: undefined,
      },
    ])
  })

  it('deduplicates identical statements', () => {
    const statement = { status: 'affected', action_statement: 'Upgrade' }

    expect(getVexNotes(withStatements(statement, { ...statement }))).toHaveLength(1)
  })

  it('handles matches without vex statements', () => {
    expect(getVexNotes(withStatements())).toEqual([])
  })

  it('reads statements from a real report', () => {
    const m = (report.matches as Match[]).find((m) => m.vulnerability.id === 'CVE-2025-71313')!

    expect(getVexNotes(m)).toEqual([
      expect.objectContaining({
        status: 'affected',
        action: expect.stringContaining('CONFIG_PCI_EPF_NTB'),
      }),
    ])
  })
})
