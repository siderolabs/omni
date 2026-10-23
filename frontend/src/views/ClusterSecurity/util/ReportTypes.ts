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
  knownExploited?: KnownExploited[]
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
}

/** An entry from the CISA Known Exploited Vulnerabilities catalog. */
export interface KnownExploited {
  cve: string
  vendorProject: string
  product: string
  dateAdded: string
  requiredAction: string
  dueDate: string
  knownRansomwareCampaignUse: string
  notes: string
  urls: string[]
  cwes: string[]
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

export type MatchDetail = PackageMatchDetail | VexMatchDetail

/** A match detail from a package matcher, e.g. "stock-matcher" or "go-module-matcher". */
export interface PackageMatchDetail {
  type: string
  matcher: string
  searchedBy: SearchedBy
  found: Found
  fix?: MatchDetailFix
}

/** A match detail added when an OpenVEX statement applies to the vulnerability. */
export interface VexMatchDetail {
  type: string
  matcher: 'openvex-matcher'
  searchedBy: VexSearchedBy
  found: VexFound
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

export interface VexSearchedBy {
  Vulnerability: string
  Product: string
  Subcomponents: string[]
}

export interface VexFound {
  Statement: VexStatement
}

export interface VexStatement {
  vulnerability: { name: string }
  timestamp: string
  products: { identifiers: { purl: string } }[]
  status: string
  justification?: string
  impact_statement?: string
  action_statement?: string
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
