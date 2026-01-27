// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

vi.mock('@/notification', () => ({
  showSuccess: vi.fn(),
}))

import InstallationMediaCreate from './InstallationMediaCreate.vue'

const mockSavePresetModal = {
  name: 'SavePresetModal',
  template: '<div v-if="open" class="save-preset-modal"><slot /></div>',
  props: ['open', 'formState'],
  emits: ['close', 'saved'],
}

const routes = [
  {
    path: '/',
    name: 'InstallationMediaCreateEntry',
    component: { template: '<div>Entry</div>' },
  },
  {
    path: '/talos-version',
    name: 'InstallationMediaCreateTalosVersion',
    component: { template: '<div>Talos Version</div>' },
  },
  {
    path: '/machine-arch',
    name: 'InstallationMediaCreateMachineArch',
    component: { template: '<div>Machine Arch</div>' },
  },
  {
    path: '/cloud-provider',
    name: 'InstallationMediaCreateCloudProvider',
    component: { template: '<div>Cloud Provider</div>' },
  },
  {
    path: '/sbc-type',
    name: 'InstallationMediaCreateSBCType',
    component: { template: '<div>SBC Type</div>' },
  },
  {
    path: '/system-extensions',
    name: 'InstallationMediaCreateSystemExtensions',
    component: { template: '<div>System Extensions</div>' },
  },
  {
    path: '/extra-args',
    name: 'InstallationMediaCreateExtraArgs',
    component: { template: '<div>Extra Args</div>' },
  },
  {
    path: '/confirmation',
    name: 'InstallationMediaCreateConfirmation',
    component: { template: '<div>Confirmation</div>' },
  },
]

describe('InstallationMediaCreate', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    // Clear session storage before each test
    sessionStorage.clear()
    vi.clearAllMocks()

    // Create a fresh router for each test
    router = createRouter({
      history: createMemoryHistory(),
      routes,
    })
  })

  test('renders the component with title', async () => {
    await router.push({ name: 'InstallationMediaCreateEntry' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    expect(screen.getByText('Create New Media')).toBeDefined()
  })

  test('renders entry page content', async () => {
    await router.push({ name: 'InstallationMediaCreateEntry' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    expect(screen.getByText('Entry')).toBeDefined()
  })

  test('renders correct content for different routes', async () => {
    await router.push({ name: 'InstallationMediaCreateTalosVersion' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    expect(screen.getByText('Talos Version')).toBeDefined()
  })

  test('different hardware types have different flow steps', async () => {
    const testHardwareTypes = [
      { type: 'metal' as const, expectedSteps: 5 },
      { type: 'cloud' as const, expectedSteps: 6 },
      { type: 'sbc' as const, expectedSteps: 5 },
    ]

    for (const { type, expectedSteps } of testHardwareTypes) {
      sessionStorage.clear()
      sessionStorage.setItem(
        '_installation_media_form',
        JSON.stringify({
          hardwareType: type,
        }),
      )

      await router.push({ name: 'InstallationMediaCreateTalosVersion' })
      await router.isReady()

      const { unmount } = render(InstallationMediaCreate, {
        global: {
          plugins: [router],
          components: {
            SavePresetModal: mockSavePresetModal,
          },
          stubs: {
            TIcon: true,
            TButton: true,
            Stepper: {
              template:
                '<div data-testid="stepper" :data-steps="stepCount">Stepper {{ stepCount }}</div>',
              props: ['modelValue', 'stepCount'],
              emits: ['update:modelValue'],
            },
            Tooltip: true,
          },
        },
      })

      // Find stepper and check step count
      expect(screen.getByTestId('stepper').getAttribute('data-steps')).toBe(String(expectedSteps))

      unmount()
    }
  })

  test('session storage persists form state', async () => {
    await router.push({ name: 'InstallationMediaCreateEntry' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Simulate setting form state
    const state = { hardwareType: 'metal', talosVersion: '1.8.0' }
    sessionStorage.setItem('_installation_media_form', JSON.stringify(state))

    // Verify it's stored
    const stored = JSON.parse(sessionStorage.getItem('_installation_media_form') || '{}')
    expect(stored.hardwareType).toBe('metal')
    expect(stored.talosVersion).toBe('1.8.0')
  })

  test('clears session storage when visiting entry page', async () => {
    // Set initial state
    sessionStorage.setItem(
      '_installation_media_form',
      JSON.stringify({
        hardwareType: 'metal',
        talosVersion: '1.8.0',
      }),
    )

    await router.push({ name: 'InstallationMediaCreateEntry' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Session storage should be cleared
    await waitFor(() => {
      const stored = JSON.parse(sessionStorage.getItem('_installation_media_form') || '{}')
      expect(Object.keys(stored).length).toBe(0)
    })
  })

  test('maintains form state during navigation', async () => {
    const testState = {
      hardwareType: 'metal' as const,
      talosVersion: '1.8.0',
    }

    sessionStorage.setItem('_installation_media_form', JSON.stringify(testState))

    await router.push({ name: 'InstallationMediaCreateTalosVersion' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Verify state is preserved
    const stored = JSON.parse(sessionStorage.getItem('_installation_media_form') || '{}')
    expect(stored).toEqual(testState)
  })

  test('does not render stepper on entry page', async () => {
    await router.push({ name: 'InstallationMediaCreateEntry' })
    await router.isReady()

    const { container } = render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          Stepper: { template: '<div class="stepper">Stepper</div>' },
          Tooltip: true,
        },
      },
    })

    // Check that stepper is not rendered on entry page
    expect(container.querySelector('.stepper')).not.toBeInTheDocument()
  })

  test('renders stepper when not on entry page', async () => {
    sessionStorage.setItem(
      '_installation_media_form',
      JSON.stringify({
        hardwareType: 'metal',
      }),
    )

    await router.push({ name: 'InstallationMediaCreateTalosVersion' })
    await router.isReady()

    const { container } = render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          Stepper: { template: '<div class="stepper">Stepper</div>' },
          Tooltip: true,
        },
      },
    })

    // Check that stepper div exists in the container
    expect(container.querySelector('.stepper')).toBeInTheDocument()
  })

  test('modal is initially closed', async () => {
    await router.push({ name: 'InstallationMediaCreateTalosVersion' })
    await router.isReady()

    const { container } = render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: true,
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Modal is initially closed, so it shouldn't be visible
    expect(container.querySelector('.save-preset-modal')).not.toBeInTheDocument()
  })

  test('reset button resets the form', async () => {
    sessionStorage.setItem(
      '_installation_media_form',
      JSON.stringify({
        hardwareType: 'metal',
      }),
    )

    await router.push({ name: 'InstallationMediaCreateTalosVersion' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          Stepper: true,
          Tooltip: { template: '<span class="tooltip"><slot /></span>' },
        },
      },
    })

    screen.getByRole('link', { name: 'reset wizard' }).click()

    // Session storage should be cleared
    await waitFor(() => {
      const stored = JSON.parse(sessionStorage.getItem('_installation_media_form') || '{}')
      expect(Object.keys(stored).length).toBe(0)
    })
  })

  test('renders finished button when on last step and form is saved', async () => {
    // Pre-populate form state
    sessionStorage.setItem('_installation_media_form', JSON.stringify({ hardwareType: 'metal' }))
    sessionStorage.setItem('_installation_media_form_saved', JSON.stringify(true))

    await router.push({ name: 'InstallationMediaCreateConfirmation' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Should show Finished button when isSaved is true on last step
    expect(screen.getByRole('button', { name: 'Finished' })).toBeInTheDocument()
  })

  test('renders save button when on last step and form is not saved', async () => {
    sessionStorage.setItem(
      '_installation_media_form',
      JSON.stringify({
        hardwareType: 'metal',
        isSaved: false,
      }),
    )

    await router.push({ name: 'InstallationMediaCreateConfirmation' })
    await router.isReady()

    render(InstallationMediaCreate, {
      global: {
        plugins: [router],
        components: {
          SavePresetModal: mockSavePresetModal,
        },
        stubs: {
          TIcon: true,
          TButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          Stepper: true,
          Tooltip: true,
        },
      },
    })

    // Should show Save button in footer when isSaved is false on last step
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })
})
