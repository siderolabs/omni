// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { compare } from 'semver'

import DownloadTalosctl from './DownloadTalosctl.vue'

const meta: Meta<typeof DownloadTalosctl> = {
  component: DownloadTalosctl,
}

export default meta
type Story = StoryObj<typeof meta>

const versions = faker.helpers
  .uniqueArray(faker.system.semver, 10)
  .sort(compare)
  .map((v) => `v${v}`)

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.get('/talosctl/downloads', () =>
          HttpResponse.json({
            release_data: {
              available_versions: versions.reduce<Record<string, { name: string; url: string }[]>>(
                (prev, curr) => ({
                  ...prev,
                  [curr]: [
                    {
                      name: 'Apple',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-darwin-amd64`,
                    },
                    {
                      name: 'Apple Silicon',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-darwin-arm64`,
                    },
                    {
                      name: 'Linux',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-linux-amd64`,
                    },
                    {
                      name: 'Linux ARM',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-linux-armv7`,
                    },
                    {
                      name: 'Linux ARM64',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-linux-arm64`,
                    },
                    {
                      name: 'Windows',
                      url: `https://github.com/siderolabs/talos/releases/download/${curr}/talosctl-windows-amd64.exe`,
                    },
                  ],
                }),
                {},
              ),
              default_version: versions.at(-1),
            },
            status: 'ok',
          }),
        ),
      ],
    },
  },
}
