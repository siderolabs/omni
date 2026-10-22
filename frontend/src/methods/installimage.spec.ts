// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import { installImageDiff } from './installimage'

test('installImageDiff marks the changed fields only', () => {
  expect(
    installImageDiff(
      { talos_version: '1.13.10', schematic_id: 'abc', image_factory_host: 'factory.talos.dev' },
      { talos_version: '1.13.11', schematic_id: 'abc', image_factory_host: 'factory.talos.dev' },
    ),
  ).toBe(
    [
      '@@ -1,3 +1,3 @@',
      '-talosVersion: 1.13.10',
      '+talosVersion: 1.13.11',
      ' schematic: abc',
      ' imageFactoryHost: factory.talos.dev',
    ].join('\n'),
  )
})

test('installImageDiff leaves out the fields empty on both sides', () => {
  expect(
    installImageDiff(
      { talos_version: '1.13.10', schematic_id: 'abc' },
      { talos_version: '1.13.11', schematic_id: 'abc' },
    ),
  ).toBe(
    ['@@ -1,2 +1,2 @@', '-talosVersion: 1.13.10', '+talosVersion: 1.13.11', ' schematic: abc'].join(
      '\n',
    ),
  )
})

test('installImageDiff shows a field empty on one side as an empty string', () => {
  expect(
    installImageDiff(
      { talos_version: '1.13.10', image_factory_host: 'factory.talos.dev' },
      { talos_version: '1.13.11', schematic_id: 'abc', image_factory_host: 'factory.talos.dev' },
    ),
  ).toBe(
    [
      '@@ -1,3 +1,3 @@',
      '-talosVersion: 1.13.10',
      '+talosVersion: 1.13.11',
      '-schematic: ""',
      '+schematic: abc',
      ' imageFactoryHost: factory.talos.dev',
    ].join('\n'),
  )
})
