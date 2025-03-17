// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { fetchOption } from "../../src/api/fetch.pb";
import { Resource, ResourceService } from "../../src/api/grpc";
import { GetRequest } from "../../src/api/omni/resources/resources.pb";
import { ExposedServiceType } from "../../src/api/resources";

import { verifyURL } from "../../src/methods/auth";

import { describe, expect, test } from "bun:test";

describe("verifyURL test", () => {
  const existing = "pwqd4t";

  ResourceService.Get = async <T extends Resource>(request: GetRequest, ...options: fetchOption[]): Promise<T> => {
    if (request.type !== ExposedServiceType) {
      throw {code: 5};
    }

    if (request.id === existing) {
      return {
        metadata: {
          id: request.id,
          namespace: request.namespace,
          type: request.type,
        },
        spec: {
          url: `https://${request.id}-user.proxy-us.siderolabs.io`,
        },
      } as T;
    }

    throw {code: 5};
  }

  const tests: {
    name: string
    url: string
    allowed: boolean
  }[] = [
    {
      name: "existing exposed service https",
      url: `https://${existing}-user.proxy-us.siderolabs.io/`,
      allowed: true
    },
    {
      name: "existing exposed service http",
      url: `http://${existing}-user.proxy-us.siderolabs.io/`,
      allowed: true
    },
    {
      name: "non existing exposed service",
      url: `https://123456-user.proxy-us.siderolabs.io/`,
      allowed: false
    },
    {
      name: "non exposed service",
      url: `https://google.com/`,
      allowed: false
    },
    {
      name: "root url",
      url: `/omni`,
      allowed: true
    },
  ];

  for (const tt of tests) {
    test(tt.name, async () => {
      expect(await verifyURL(tt.url)).toBe(tt.allowed);
    })
  }
})
