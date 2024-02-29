/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
export type AuthenticateRequest = {
  auth_request_id?: string
}

export type AuthenticateResponse = {
  redirect_url?: string
}

export class OIDCService {
  static Authenticate(req: AuthenticateRequest, ...options: fm.fetchOption[]): Promise<AuthenticateResponse> {
    return fm.fetchReq<AuthenticateRequest, AuthenticateResponse>("POST", `/oidc.OIDCService/Authenticate`, req, ...options)
  }
}