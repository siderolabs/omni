// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { expect, test } from 'vitest'

import ClusterEtcdBackupCheckbox from './ClusterEtcdBackupCheckbox.vue'

const backupStatus = { enabled: true, configurable: true }

test('lets the interval preview be edited to a different value', async () => {
  const user = userEvent.setup()

  render(ClusterEtcdBackupCheckbox, {
    props: {
      backupStatus,
      cluster: { backup_configuration: { enabled: true, interval: '3600s' } },
    },
  })

  await user.click(screen.getByRole('button'))

  const input = screen.getByRole('spinbutton')

  expect(input).toHaveValue(1)

  await user.clear(input)
  await user.type(input, '5')

  expect(input).toHaveValue(5)
})
