// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type {
  Match,
  RelatedVulnerability,
  Vulnerability,
} from '@/views/InstallationMedia/vulnerabilities/ReportTypes'

// Severities ordered from most to least important, used for sorting and grouping.
const SEVERITY_ORDER = ['Critical', 'High', 'Medium', 'Low', 'Negligible', 'Unknown'] as const

function severityRank(severity: string): number {
  const idx = SEVERITY_ORDER.indexOf(severity as (typeof SEVERITY_ORDER)[number])

  return idx === -1 ? SEVERITY_ORDER.length : idx
}

/** The CVSS base score for a match, preferring the primary CVSS entry. */
export function getScore(m: Match): number | undefined {
  return (
    m.vulnerability.cvss.find((c) => c.type === 'Primary')?.metrics.baseScore ||
    m.vulnerability.cvss.at(0)?.metrics.baseScore
  )
}

/** The human-facing CVE identifier for a match, falling back to a related CVE or the raw id. */
export function getCveId(m: Match): string {
  if (m.vulnerability.id.startsWith('CVE-')) return m.vulnerability.id

  return m.relatedVulnerabilities.find((r) => r.id.startsWith('CVE-'))?.id || m.vulnerability.id
}

function compileVulnURLs(v: Vulnerability) {
  return [v.dataSource, ...(v.urls ?? [])].filter((s): s is string => !!s)
}

export function getPreferredVulnURL(
  v: Vulnerability,
  relatedVulnerabilities: RelatedVulnerability[],
) {
  const pool: string[] = []

  if (v.id.startsWith('CVE-')) pool.push(...compileVulnURLs(v))

  if (relatedVulnerabilities)
    pool.push(
      ...relatedVulnerabilities.filter((r) => r.id.startsWith('CVE-')).flatMap(compileVulnURLs),
    )

  if (!pool.length) pool.push(...compileVulnURLs(v))

  return pool.find((p) => p.includes('nvd.nist.gov')) || pool[0] || ''
}

/**
 * A stable identity for a finding across Talos versions: the CVE plus the affected
 * package name. Package versions are intentionally excluded so the same CVE in the
 * same package is recognised as "the same finding" even when the package is bumped.
 */
export function matchKey(m: Match): string {
  return `${getCveId(m)}::${m.artifact.name}`
}

/** Sort matches by CVSS score (desc), then severity, then CVE id for stable ordering. */
export function sortMatches(matches: Match[]): Match[] {
  return matches.toSorted((a, b) => {
    const scoreA = getScore(a)
    const scoreB = getScore(b)

    if (scoreA && scoreB && scoreA !== scoreB) return scoreB - scoreA

    const sevDiff = severityRank(a.vulnerability.severity) - severityRank(b.vulnerability.severity)
    if (sevDiff !== 0) return sevDiff

    return getCveId(b).localeCompare(getCveId(a))
  })
}

/** Count matches by severity, preserving severity ordering. */
export function countBySeverity(matches: Match[]): Map<string, number> {
  const counts = new Map<string, number>()

  for (const m of matches) {
    const sev = m.vulnerability.severity
    counts.set(sev, (counts.get(sev) ?? 0) + 1)
  }

  return new Map([...counts.entries()].sort(([a], [b]) => severityRank(a) - severityRank(b)))
}
