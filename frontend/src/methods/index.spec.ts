// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect } from '@playwright/test'
import { parse } from 'semver'
import { test } from 'vitest'

import { DefaultTalosVersion } from '@/api/resources'

import { getDocsLink } from '.'

const { major, minor } = parse(DefaultTalosVersion, false, true)
const defaultVersion = `${major}.${minor}`

test.each`
  type       | path            | options                       | expected
  ${'talos'} | ${undefined}    | ${undefined}                  | ${`https://docs.siderolabs.com/talos/v${defaultVersion}`}
  ${'talos'} | ${'/hello/123'} | ${undefined}                  | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello/123`}
  ${'talos'} | ${'hello/123'}  | ${undefined}                  | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello/123`}
  ${'talos'} | ${'/hello'}     | ${undefined}                  | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello`}
  ${'talos'} | ${'123'}        | ${undefined}                  | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/123`}
  ${'talos'} | ${'/hello/123'} | ${{}}                         | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello/123`}
  ${'talos'} | ${'hello/123'}  | ${{}}                         | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello/123`}
  ${'talos'} | ${'/hello'}     | ${{}}                         | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/hello`}
  ${'talos'} | ${'123'}        | ${{}}                         | ${`https://docs.siderolabs.com/talos/v${defaultVersion}/123`}
  ${'talos'} | ${'/hello/123'} | ${{ talosVersion: '1.11.3' }} | ${'https://docs.siderolabs.com/talos/v1.11/hello/123'}
  ${'talos'} | ${'hello/123'}  | ${{ talosVersion: '1.10.0' }} | ${'https://docs.siderolabs.com/talos/v1.10/hello/123'}
  ${'talos'} | ${'/hello'}     | ${{ talosVersion: '1.9' }}    | ${'https://docs.siderolabs.com/talos/v1.9/hello'}
  ${'talos'} | ${'123'}        | ${{ talosVersion: '1' }}      | ${'https://docs.siderolabs.com/talos/v1.0/123'}
  ${'omni'}  | ${undefined}    | ${undefined}                  | ${'https://docs.siderolabs.com/omni'}
  ${'omni'}  | ${'/hello/123'} | ${undefined}                  | ${'https://docs.siderolabs.com/omni/hello/123'}
  ${'omni'}  | ${'hello/123'}  | ${undefined}                  | ${'https://docs.siderolabs.com/omni/hello/123'}
  ${'omni'}  | ${'/hello'}     | ${undefined}                  | ${'https://docs.siderolabs.com/omni/hello'}
  ${'omni'}  | ${'123'}        | ${undefined}                  | ${'https://docs.siderolabs.com/omni/123'}
  ${'k8s'}   | ${undefined}    | ${undefined}                  | ${'https://docs.siderolabs.com/kubernetes-guides'}
  ${'k8s'}   | ${'/hello/123'} | ${undefined}                  | ${'https://docs.siderolabs.com/kubernetes-guides/hello/123'}
  ${'k8s'}   | ${'hello/123'}  | ${undefined}                  | ${'https://docs.siderolabs.com/kubernetes-guides/hello/123'}
  ${'k8s'}   | ${'/hello'}     | ${undefined}                  | ${'https://docs.siderolabs.com/kubernetes-guides/hello'}
  ${'k8s'}   | ${'123'}        | ${undefined}                  | ${'https://docs.siderolabs.com/kubernetes-guides/123'}
`(
  'getDocsLink: $type - $path - $options returns $expected',
  ({ type, path, options, expected }) => {
    expect(getDocsLink(type, path, options)).toBe(expected)
  },
)
