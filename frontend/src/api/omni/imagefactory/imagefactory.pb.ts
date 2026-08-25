/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"

export enum Arch {
  UNKNOWN_ARCH = 0,
  AMD64 = 1,
  ARM64 = 2,
}

export enum VulnerabilityReportFormat {
  UNKNOWN_FORMAT = 0,
  JSON = 1,
  SARIF = 2,
  CYCLONEDX = 3,
  TABLE = 4,
}

export type VulnerabilityReportRequest = {
  schematic_id?: string
  talos_version?: string
  arch?: Arch
  format?: VulnerabilityReportFormat
}

export type VulnerabilityReportResponse = {
  data?: Uint8Array
}

export type SBOMRequest = {
  schematic_id?: string
  talos_version?: string
  arch?: Arch
}

export type SBOMResponse = {
  data?: Uint8Array
}

export type VEXDocumentRequest = {
  talos_version?: string
}

export type VEXDocumentResponse = {
  data?: Uint8Array
}

export class ImageFactoryService {
  static VulnerabilityReport(req: VulnerabilityReportRequest, ...options: fm.fetchOption[]): Promise<VulnerabilityReportResponse> {
    return fm.fetchReq<VulnerabilityReportRequest, VulnerabilityReportResponse>("POST", `/imagefactory.ImageFactoryService/VulnerabilityReport`, req, ...options)
  }
  static SBOM(req: SBOMRequest, ...options: fm.fetchOption[]): Promise<SBOMResponse> {
    return fm.fetchReq<SBOMRequest, SBOMResponse>("POST", `/imagefactory.ImageFactoryService/SBOM`, req, ...options)
  }
  static VEXDocument(req: VEXDocumentRequest, ...options: fm.fetchOption[]): Promise<VEXDocumentResponse> {
    return fm.fetchReq<VEXDocumentRequest, VEXDocumentResponse>("POST", `/imagefactory.ImageFactoryService/VEXDocument`, req, ...options)
  }
}