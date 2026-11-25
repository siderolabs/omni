// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { fireEvent, render, screen } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { Resource } from '@/api/grpc'
import {
  type ClusterMachineStatusSpec,
  type DryRunStatusSpec,
  DryRunStatusSpecNotCheckedReason,
  DryRunStatusSpecOutcome,
} from '@/api/omni/specs/omni.pb'
import { LabelCluster, LabelWorkerRole, MachineLocked, UpdateLocked } from '@/api/resources'
import { updateMachineLock } from '@/methods/machine'

import ClusterMachine from './ClusterMachine.vue'
import ClusterMachineDryRunDescription from './ClusterMachineDryRunDescription.vue'
import ClusterMachineDryRunResult from './ClusterMachineDryRunResult.vue'

const push = vi.fn()

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push }),
}))
vi.mock('@/methods/machine', () => ({ updateMachineLock: vi.fn() }))

beforeEach(() => {
  vi.clearAllMocks()
})

// with an href, so the link can take focus
const RouterLink = { template: '<a href="#"><slot /></a>' }

const renderResult = (dryRun: DryRunStatusSpec) =>
  render(ClusterMachineDryRunResult, {
    props: { dryRun, to: '/' },
    global: { stubs: { RouterLink } },
  })

describe('the description', () => {
  test.each<[string, DryRunStatusSpec, string]>([
    ['failed', { outcome: DryRunStatusSpecOutcome.FAILED }, 'Pending config would fail'],
    ...(
      [
        [DryRunStatusSpecNotCheckedReason.DISCONNECTED, 'disconnected'],
        [DryRunStatusSpecNotCheckedReason.TRY_IN_FLIGHT, 'config try in flight'],
        [DryRunStatusSpecNotCheckedReason.MAINTENANCE, 'maintenance mode'],
        [DryRunStatusSpecNotCheckedReason.NOT_RUNNING, 'not running'],
        [DryRunStatusSpecNotCheckedReason.CHECK_ERROR, 'check failed'],
      ] as const
    ).map(([reason, text]): [string, DryRunStatusSpec, string] => [
      `not checked: ${text}`,
      {
        outcome: DryRunStatusSpecOutcome.NOT_CHECKED,
        not_checked_reason: reason,
      },
      `Not checked: ${text}`,
    ]),
  ])('%s', (_, dryRun, summary) => {
    render(ClusterMachineDryRunDescription, { props: { dryRun } })

    expect(screen.getByText(summary)).toBeInTheDocument()
    expect(screen.getByText('Nothing has been applied.')).toBeInTheDocument()
  })

  test('a failed dry run shows the error as it is', () => {
    const error =
      '2 errors occurred:\n\t* disk selector is required\n\t* min size is greater than max size'

    render(ClusterMachineDryRunDescription, {
      props: { dryRun: { outcome: DryRunStatusSpecOutcome.FAILED, error } },
    })

    expect(screen.queryByText('The dry run could not run:')).not.toBeInTheDocument()
    expect(screen.getByText(/disk selector is required/).textContent).toBe(error)
  })

  test('a check error is not shown as a failure of the config', () => {
    render(ClusterMachineDryRunDescription, {
      props: {
        dryRun: {
          outcome: DryRunStatusSpecOutcome.NOT_CHECKED,
          not_checked_reason: DryRunStatusSpecNotCheckedReason.CHECK_ERROR,
          error: 'context deadline exceeded',
        },
      },
    })

    expect(screen.getByText('The dry run could not run:')).toBeInTheDocument()
    expect(screen.getByText('context deadline exceeded')).toBeInTheDocument()
  })
})

describe('the result', () => {
  test.each<[string, DryRunStatusSpec, string, string, string]>([
    [
      'passed',
      { outcome: DryRunStatusSpecOutcome.PASSED },
      'would apply',
      'success',
      'text-status-success-default',
    ],
    [
      'passed with a reboot',
      { outcome: DryRunStatusSpecOutcome.PASSED, reboot_required: true },
      'would reboot',
      'success',
      'text-status-success-default',
    ],
    [
      'failed',
      { outcome: DryRunStatusSpecOutcome.FAILED },
      'would fail',
      'warning',
      'text-status-warning-default',
    ],
    [
      'not checked',
      { outcome: DryRunStatusSpecOutcome.NOT_CHECKED },
      'not checked',
      'unknown',
      'text-status-info-default',
    ],
  ])('%s', (_, dryRun, word, glyph, tone) => {
    renderResult(dryRun)

    expect(screen.getByTestId('cluster-machine-dry-run')).toHaveTextContent(word)
    expect(screen.getByTestId(`cluster-machine-dry-run-${glyph}`)).toHaveClass(tone)
  })

  test.each<[string, DryRunStatusSpecOutcome | undefined]>([
    ['no pending changes', DryRunStatusSpecOutcome.NO_PENDING_CHANGES],
    ['unknown', undefined],
  ])('%s shows nothing', (_, outcome) => {
    renderResult({ outcome })

    expect(screen.queryByTestId('cluster-machine-dry-run')).not.toBeInTheDocument()
  })

  test('the reason it was not checked shows on keyboard focus', async () => {
    renderResult({
      outcome: DryRunStatusSpecOutcome.NOT_CHECKED,
      not_checked_reason: DryRunStatusSpecNotCheckedReason.DISCONNECTED,
    })

    screen.getByTestId('cluster-machine-dry-run').focus()

    expect(await screen.findAllByText('Not checked: disconnected')).not.toHaveLength(0)
  })
})

describe('the machine row', () => {
  const machine = (labels: Record<string, string>): Resource<ClusterMachineStatusSpec> => ({
    metadata: {
      id: 'machine-1',
      labels: { [LabelCluster]: 'cluster', ...labels },
      annotations: { [MachineLocked]: '' },
    },
    spec: {},
  })

  const renderRow = (
    labels: Record<string, string>,
    dryRun: DryRunStatusSpec = { outcome: DryRunStatusSpecOutcome.FAILED, error: 'invalid' },
    clusterLocked = false,
    lockedOnItsOwn = false,
  ) =>
    render(ClusterMachine, {
      props: { machine: machine(labels), dryRun, clusterLocked, lockedOnItsOwn },
      global: {
        stubs: {
          RouterLink,
          ClusterMachinePhase: true,
          NodeContextMenu: true,
        },
      },
    })

  test('the result follows the pending config update link', () => {
    renderRow({ [UpdateLocked]: '' })

    const result = screen.getByTestId('cluster-machine-dry-run')

    expect(result).toHaveTextContent('would fail')
    expect(result).not.toHaveTextContent('Pending config')
  })

  test('the result is a link that does not open the node', async () => {
    renderRow({})

    const result = screen.getByTestId('cluster-machine-dry-run')

    expect(result.tagName).toBe('A')

    await fireEvent.click(result)

    expect(push).not.toHaveBeenCalled()
  })

  test('a failure without pending updates reads on its own', () => {
    renderRow({})

    expect(screen.getByTestId('cluster-machine-dry-run')).toHaveTextContent(
      'Pending config would fail',
    )
  })

  test('a passed dry run without pending updates shows nothing', () => {
    renderRow({}, { outcome: DryRunStatusSpecOutcome.PASSED })

    expect(screen.queryByTestId('cluster-machine-dry-run')).not.toBeInTheDocument()
  })

  test('a control plane has no lock', () => {
    renderRow({})

    expect(screen.queryByTestId('cluster-machine-lock')).not.toBeInTheDocument()
  })

  test('a worker locked on its own in a locked cluster shows its lock disabled', async () => {
    renderRow({ [LabelWorkerRole]: '' }, undefined, true, true)

    expect(screen.getByTestId('cluster-machine-lock')).toBeDisabled()

    const lock = screen.getByRole('img', { name: 'Locked on its own' })

    await fireEvent.click(lock)

    expect(push).not.toHaveBeenCalled()

    lock.focus()

    expect(
      await screen.findAllByText(
        'The machine is locked on its own. Unlocking the cluster does not unlock it.',
      ),
    ).not.toHaveLength(0)
  })

  test('the lock of a worker in an unlocked cluster shows its hint on keyboard focus', async () => {
    renderRow({ [LabelWorkerRole]: '' })

    screen.getByTestId('cluster-machine-lock').focus()

    expect(
      await screen.findAllByText(
        'Lock machine config. Pause Kubernetes and Talos updates on the machine.',
      ),
    ).not.toHaveLength(0)
  })

  test('a worker not locked on its own in a locked cluster has no lock', () => {
    renderRow({ [LabelWorkerRole]: '' }, undefined, true)

    expect(screen.queryByTestId('cluster-machine-lock')).not.toBeInTheDocument()
  })

  test('unlocking a worker with a passed dry run does not ask', async () => {
    renderRow({ [LabelWorkerRole]: '' }, { outcome: DryRunStatusSpecOutcome.PASSED })

    await fireEvent.click(screen.getByTestId('cluster-machine-lock'))

    expect(updateMachineLock).toHaveBeenCalledWith('machine-1', false)
    expect(push).not.toHaveBeenCalled()
  })

  test('unlocking a worker with a failed dry run asks first', async () => {
    renderRow({ [LabelWorkerRole]: '' })

    await fireEvent.click(screen.getByTestId('cluster-machine-lock'))

    expect(await screen.findByText('Unlock machine machine-1?')).toBeInTheDocument()
    expect(screen.getByTestId('dry-run-unlock-failure')).toHaveTextContent('invalid')
    expect(updateMachineLock).not.toHaveBeenCalled()

    await fireEvent.click(screen.getByRole('button', { name: 'Unlock machine' }))

    expect(updateMachineLock).toHaveBeenCalledWith('machine-1', false)
  })
})
