// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import {
  createMessage,
  enums,
  generateKey,
  Key,
  PrivateKey,
  readKey,
  readPrivateKey,
  sign
} from 'openpgp/lightweight';
import { AuthService } from '@/api/omni/auth/auth.pb';
import * as fetchIntercept from 'fetch-intercept';
import {
  authHeader,
  PayloadHeaderKey,
  SignatureHeaderKey,
  TimestampHeaderKey,
  SignatureVersionV1,
  workloadProxyPublicKeyIdCookie,
  workloadProxyPublicKeyIdSignatureBase64Cookie,
} from '@/api/resources';
import { ref } from 'vue';
import { b64Encode } from '@/api/fetch.pb';

let interceptorsRegistered = false;
let keysReloadTimeout: NodeJS.Timeout;

export let keys: {
  privateKey: PrivateKey,
  publicKey: Key,
  identity: string,
} | null;

export class KeysInvalidError extends Error {
  constructor(msg: string) {
    super(msg);

    // Set the prototype explicitly.
    Object.setPrototypeOf(this, KeysInvalidError.prototype);
  }
}

export const authorized = ref(false);

export const isAuthorized = async (): Promise<boolean> => {
  if (!keys) {
    try {
      await loadKeys();
    } catch {
      return false;
    }
  }

  const keyExpirationTime = await keys?.privateKey.getExpirationTime();

  if (!keyExpirationTime || new Date() > keyExpirationTime) {
    return false;
  }

  return true;
}

export const loadKeys = async (): Promise<{privateKey: PrivateKey, publicKey: Key}> => {
  if (!keys) {
    const privateKeyArmored = window.localStorage.getItem("privateKey");
    const publicKeyArmored = window.localStorage.getItem("publicKey");
    const identity = window.localStorage.getItem("identity");

    if (!privateKeyArmored || !publicKeyArmored || !identity) {
      throw new KeysInvalidError(`failed to load keys: keys not initialized`);
    }

    keys = {
      privateKey: await readPrivateKey({ armoredKey: privateKeyArmored }),
      publicKey: await readKey({ armoredKey: publicKeyArmored }),
      identity: identity.toLowerCase(),
    }
  }

  const now = new Date();

  const keyExpirationTime = await keys.privateKey.getExpirationTime();

  if (!keyExpirationTime || now > keyExpirationTime) {
    throw new KeysInvalidError(`failed to load keys: the key is expired`);
  }

  registerInterceptors();

  clearTimeout(keysReloadTimeout)
  keysReloadTimeout = setTimeout(() => {
    location.reload();
  }, (keyExpirationTime as Date).getTime() - now.getTime() + 1000);

  authorized.value = true;

  return keys;
}

export const createKeys = async (email: string): Promise<{privateKey: string, publicKey: string, publicKeyId: string}> => {
  email = email.toLowerCase();

  const res = await genKey(email);

  const enc = new TextEncoder();

  const response = await AuthService.RegisterPublicKey({
    public_key: {
      pgp_data: enc.encode(res.publicKey),
    },
    identity: {
      email: email,
    },
  });

  return {
    publicKeyId: response.public_key_id!,
    ...res,
  };
}

export const signDetached = async (data: string): Promise<string> => {
  const keys = await loadKeys();

  const stream = await sign({
    message: await createMessage({ text: data }),
    detached: true,
    signingKeys: keys.privateKey,
    format: 'binary',
  });

  const array = stream as Uint8Array;

  return b64Encode(array, 0, array.length);
}

export const saveKeys = async (user: {email: string, picture: string, fullname: string}, privateKey: string, publicKey: string, publicKeyId: string) => {
  keys = null;

  window.localStorage.setItem("publicKey", publicKey);
  window.localStorage.setItem("privateKey", privateKey);
  window.localStorage.setItem("identity", user.email.toLowerCase());
  window.localStorage.setItem("avatar", user.picture);
  window.localStorage.setItem("fullname", user.fullname);

  const loadedKeys = await loadKeys();

  const expirationTime = await loadedKeys.privateKey.getExpirationTime();

  if (!(expirationTime instanceof Date)) {
    throw new KeysInvalidError("failed to save keys: invalid expiration time");
  }

  await saveAuthCookies(publicKeyId, expirationTime);
}

export const resetKeys = () => {
  keys = null;

  window.localStorage.removeItem("publicKey");
  window.localStorage.removeItem("privateKey");
  window.localStorage.removeItem("identity");
  window.localStorage.removeItem("avatar");
  window.localStorage.removeItem("fullname");

  removeAuthCookies();

  authorized.value = false;
}

const genKey = async (email: string): Promise<{publicKey: string, privateKey: string}> => {
  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519',
    userIDs: [{ email: email.toLowerCase() }, ],
    keyExpirationTime: 7 * 60 * 60 + 50 * 60, // 7 hours 50 minutes
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    }
  });

  return {
    publicKey: publicKey, privateKey: privateKey
  };
}

const includedHeaders = [
  "nodes",
  "selectors",
  "fieldSelectors",
  "runtime",
  "context",
  "cluster",
  "namespace",
  "uid",
  TimestampHeaderKey,
  authHeader,
];

const buildPayload = (url: string, config: {headers?: Headers }): {headers: Record<string, string[]>, method: string} => {
  const headers: Record<string, string[]> = {};

  if (config.headers) {
    for (const header of includedHeaders) {
      const key = `Grpc-Metadata-${header}`;
      const value = config.headers.get(key);

      if (value) {
        if (!headers[header]) {
          headers[header] = [value];
        } else {
          headers[header].push(value);
        }
      }
    }
  }

  return {
    headers: headers,
    method: url.replace(/^\/api/, ""),
  }
};

const registerInterceptors = () => {
  if (interceptorsRegistered) {
    return;
  }

  fetchIntercept.register({
    request: async (url, config?: {headers?: Headers, method?: string}) => {
      url = encodeURI(url);

      if (!/^\/(api|image)/.test(url) || url.indexOf("/api/auth.") != -1) {
        return [url, config];
      }

      if (!config) {
        config = {};
      }

      if (!config.headers) {
        config.headers = new Headers();
      }

      const ts = (new Date().getTime() / 1000).toFixed(0);

      try {
        if (url.indexOf("/api") == 0) {
          config.headers.set(`Grpc-Metadata-${TimestampHeaderKey}`, ts);

          const payload = JSON.stringify(buildPayload(url, config));
          const signature = await signDetached(payload);
          const fingerprint = keys?.publicKey.getFingerprint();

          config.headers.set(`Grpc-Metadata-${PayloadHeaderKey}`, payload);
          config.headers.set(`Grpc-Metadata-${SignatureHeaderKey}`, `${SignatureVersionV1} ${keys?.identity} ${fingerprint} ${signature}`);
        } else if (url.indexOf("/image/") == 0) {
          config.headers.set(TimestampHeaderKey, ts);

          const sha256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"; // empty string sha256
          const payload = [config.method ?? "GET", url, ts, sha256].join("\n");
          const signature = await signDetached(payload);
          const fingerprint = keys?.publicKey.getFingerprint();

          config.headers.set(SignatureHeaderKey, `${SignatureVersionV1} ${keys?.identity} ${fingerprint} ${signature}`);
        }
      } catch {
        // reload the page to make the key Authenticator regenerate the key
        location.reload();
      }

      return [url, config];
    },
  });

  interceptorsRegistered = true;
}

export const getParentDomain = () => {
  const domainParts = window.location.hostname.split('.');
  if (domainParts.length < 2) {
    console.error("there is no parent domain for the current hostname, returning the current hostname");

    return window.location.hostname;
  }

  return domainParts.slice(1).join('.')
}

export const getAuthCookies = (): {publicKeyId: string, publicKeyIdSignatureBase64: string} | undefined => {
  const getCookie = (name: string): string | undefined => {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);

    if (parts.length !== 2) {
      return undefined;
    }

    return parts.pop()!.split(';').shift();
  }

  const publicKeyIdCookie = getCookie(workloadProxyPublicKeyIdCookie);
  const publicKeyIdSignatureBase64Cookie = getCookie(workloadProxyPublicKeyIdSignatureBase64Cookie);

  if (!publicKeyIdCookie || !publicKeyIdSignatureBase64Cookie) {
    return undefined;
  }

  return {
    publicKeyId: publicKeyIdCookie,
    publicKeyIdSignatureBase64: publicKeyIdSignatureBase64Cookie,
  }
}

const saveAuthCookies = async (publicKeyId: string, expirationTime: Date) => {
  const publicKeyIdSignatureBase64 = await signDetached(publicKeyId);

  const parentDomain = getParentDomain()

  window.document.cookie = `${workloadProxyPublicKeyIdCookie}=${publicKeyId}; path=/; expires=${expirationTime.toUTCString()}; domain=.${parentDomain}`;
  window.document.cookie = `${workloadProxyPublicKeyIdSignatureBase64Cookie}=${publicKeyIdSignatureBase64}; path=/; expires=${expirationTime.toUTCString()}; domain=.${parentDomain}`;
}

const removeAuthCookies = () => {
  const parentDomain = getParentDomain()

  window.document.cookie = `${workloadProxyPublicKeyIdCookie}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 UTC; domain=.${parentDomain}`;
  window.document.cookie = `${workloadProxyPublicKeyIdSignatureBase64Cookie}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 UTC; domain=.${parentDomain}`;
}
