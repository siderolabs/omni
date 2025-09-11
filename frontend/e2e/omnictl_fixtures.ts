// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { execFile } from 'node:child_process'
import { chmod } from 'node:fs/promises'
import os from 'node:os'

import { expect, test as base } from '@playwright/test'

interface OmnictlFixtures {
  omnictl(
    args: string[],
    options?: {
      onStdout?: (chunk: string) => void
      onStderr?: (chunk: string) => void
    },
  ): Promise<{
    stdout: string
    stderr: string
  }>
}

const test = base.extend<OmnictlFixtures>({
  omnictl: async ({ page }, use, testInfo) => {
    const controller = new AbortController()
    const { signal } = controller

    await page.goto('/')

    await page.getByRole('button', { name: 'Download omnictl' }).click()

    await page.getByRole('button', { name: 'omnictl:' }).click()
    await page.getByRole('option', { name: `omnictl-${getPlatform()}-${getArch()}` }).click()

    const [downloadOmnictl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Download', exact: true }).click(),
    ])

    const omnictlPath = testInfo.outputPath(downloadOmnictl.suggestedFilename())
    await downloadOmnictl.saveAs(omnictlPath)
    await chmod(omnictlPath, 0o755)

    const [downloadOmniConfig] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Download omniconfig' }).click(),
    ])

    const omniconfigPath = testInfo.outputPath(downloadOmniConfig.suggestedFilename())
    await downloadOmniConfig.saveAs(omniconfigPath)
    await testInfo.attach(downloadOmniConfig.suggestedFilename(), { path: omniconfigPath })

    await base.step('Authenticate omnictl', async () => {
      await omnictl(['get', 'sysversion', '-ojsonpath={.spec.backendversion}'], {
        async onStderr(stderr) {
          // Go through the CLI auth flow if we get an error about requiring authentication
          if (!stderr.includes('Please visit this page to authenticate')) return

          const [authURL] = stderr.match(/\bhttps?:\/\/\S+/gi) ?? []

          if (authURL) {
            await page.goto(authURL)

            await page.getByRole('button', { name: 'Grant Access' }).click()
            await page.getByText('Successfully logged in').waitFor()
          }
        },
      })
    })

    await use(omnictl)

    controller.abort()

    function omnictl(
      args: string[],
      {
        onStdout,
        onStderr,
      }: {
        onStdout?: (chunk: string) => void
        onStderr?: (chunk: string) => void
      } = {},
    ) {
      return new Promise<{
        stdout: string
        stderr: string
      }>((resolve, reject) => {
        const commandArgs = [
          '--omniconfig',
          omniconfigPath,
          '--siderov1-keys-dir',
          testInfo.outputPath('.talos', 'keys'),
          '--insecure-skip-tls-verify',
          ...args,
        ]

        console.log([omnictlPath, ...commandArgs].join(' '))

        const child = execFile(
          omnictlPath,
          commandArgs,
          // BROWSER=none to disable the automatic opening of a browser
          // We want to do any browser actions through playwright
          { signal, encoding: 'utf-8', env: { BROWSER: 'none' } },
          (error, stdout, stderr) => {
            if (error) {
              console.error(error)
              reject(error)
              return
            }

            resolve({ stdout, stderr })
          },
        )

        child.stdout?.on('data', (chunk) => {
          console.log(chunk)
          onStdout?.(chunk)
        })

        child.stderr?.on('data', (chunk) => {
          console.warn(chunk)
          onStderr?.(chunk)
        })

        child.on('error', (err) => {
          console.error(err)
          reject(err)
        })
      })
    }
  },
})

function getPlatform() {
  switch (os.platform()) {
    case 'darwin':
      return 'darwin'
    case 'win32':
      return 'windows'
    default:
      return 'linux'
  }
}

function getArch() {
  switch (os.arch()) {
    case 'arm64':
      return 'arm64'
    default:
      return 'amd64'
  }
}

export { expect, test }
