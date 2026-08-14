// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
export interface VulnerabilityReport {
  matches: Match[]
  source: Source
  distro: Distro
  descriptor: Descriptor
}

export interface Match {
  vulnerability: Vulnerability
  relatedVulnerabilities: RelatedVulnerability[]
  matchDetails: MatchDetail[]
  artifact: Artifact
}

export interface Vulnerability {
  id: string
  dataSource: string
  namespace: string
  severity: string
  urls: string[]
  description: string
  cvss: Cvss[]
  epss?: Epss[]
  cwes?: Cwe[]
  fix: Fix
  advisories: unknown[]
  risk: number
}

export interface RelatedVulnerability {
  id: string
  dataSource: string
  namespace: string
  severity: string
  urls: string[]
  description: string
  cvss: Cvss[]
  epss?: Epss[]
  cwes?: Cwe[]
  fix: Fix
  advisories: unknown[]
  risk: number
}

export interface Cvss {
  source?: string
  type: string
  version: string
  vector: string
  metrics: CvssMetrics
  vendorMetadata: Record<string, unknown>
}

export interface CvssMetrics {
  baseScore: number
  exploitabilityScore?: number
  impactScore?: number
}

export interface Epss {
  cve: string
  epss: number
  percentile: number
  date: string
}

export interface Cwe {
  cve: string
  cwe: string
  source: string
  type: string
}

export interface Fix {
  versions: string[]
  state: string
  available?: FixAvailable[]
}

export interface FixAvailable {
  version: string
  date: string
  kind: string
}

export interface MatchDetail {
  type: string
  matcher: string
  searchedBy: SearchedBy
  found: Found
  fix?: MatchDetailFix
}

export interface SearchedBy {
  language?: string
  namespace?: string
  cpes?: string[]
  package: SearchedByPackage
}

export interface SearchedByPackage {
  name: string
  version: string
}

export interface Found {
  vulnerabilityID: string
  versionConstraint: string
  cpes?: string[]
}

export interface MatchDetailFix {
  suggestedVersion: string
}

export interface Artifact {
  id: string
  name: string
  version: string
  type: string
  locations: null
  language: string
  licenses: string[]
  cpes: string[]
  purl: string
  upstreams: unknown[]
  metadataType?: string
  metadata?: ArtifactMetadata
}

export interface ArtifactMetadata {
  goCompiledVersion: string
  architecture: string
}

export interface Source {
  type: string
  target: string
}

export interface Distro {
  name: string
  version: string
  idLike: null
}

export interface Descriptor {
  name: string
  version: string
  timestamp: string
}
