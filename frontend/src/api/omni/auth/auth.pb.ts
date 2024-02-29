/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
import * as GoogleProtobufEmpty from "../../google/protobuf/empty.pb"
export type PublicKey = {
  pgp_data?: Uint8Array
  webauthn_data?: Uint8Array
}

export type Identity = {
  email?: string
}

export type RegisterPublicKeyRequest = {
  public_key?: PublicKey
  identity?: Identity
  role?: string
  skip_user_role?: boolean
}

export type RegisterPublicKeyResponse = {
  login_url?: string
  public_key_id?: string
}

export type AwaitPublicKeyConfirmationRequest = {
  public_key_id?: string
}

export type ConfirmPublicKeyRequest = {
  public_key_id?: string
}

export class AuthService {
  static RegisterPublicKey(req: RegisterPublicKeyRequest, ...options: fm.fetchOption[]): Promise<RegisterPublicKeyResponse> {
    return fm.fetchReq<RegisterPublicKeyRequest, RegisterPublicKeyResponse>("POST", `/auth.AuthService/RegisterPublicKey`, req, ...options)
  }
  static AwaitPublicKeyConfirmation(req: AwaitPublicKeyConfirmationRequest, ...options: fm.fetchOption[]): Promise<GoogleProtobufEmpty.Empty> {
    return fm.fetchReq<AwaitPublicKeyConfirmationRequest, GoogleProtobufEmpty.Empty>("POST", `/auth.AuthService/AwaitPublicKeyConfirmation`, req, ...options)
  }
  static ConfirmPublicKey(req: ConfirmPublicKeyRequest, ...options: fm.fetchOption[]): Promise<GoogleProtobufEmpty.Empty> {
    return fm.fetchReq<ConfirmPublicKeyRequest, GoogleProtobufEmpty.Empty>("POST", `/auth.AuthService/ConfirmPublicKey`, req, ...options)
  }
}