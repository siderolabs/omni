// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import { type HardwareType, useFormState } from '@/views/omni/InstallationMedia/useFormState'

import InstallationMediaCreate from './InstallationMediaCreate.vue'

vi.mock('@/views/omni/InstallationMedia/useFormState', () => ({
  useFormState: vi.fn(),
}))

vi.mock('@/notification', () => ({
  showSuccess: vi.fn(),
}))

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

    vi.mocked(useFormState).mockReturnValue({
      formState: ref({}),
      isStepValid: vi.fn().mockReturnValue(true),
    })

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

  test.each<{ type: HardwareType; expectedSteps: number }>([
    { type: 'metal', expectedSteps: 5 },
    { type: 'cloud', expectedSteps: 6 },
    { type: 'sbc', expectedSteps: 5 },
  ])('hardware type $type should have $expectedSteps steps', async ({ type, expectedSteps }) => {
    useFormState().formState.value = {
      hardwareType: type,
    }

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
  })

  test('maintains form state during navigation', async () => {
    const testState = {
      hardwareType: 'metal' as const,
      talosVersion: '1.8.0',
    }

    useFormState().formState.value = testState

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
    expect(useFormState().formState.value).toEqual(testState)
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
    useFormState().formState.value = {
      hardwareType: 'metal',
    }

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
    useFormState().formState.value = {
      hardwareType: 'metal',
    }

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

    screen.getByRole('button', { name: 'reset wizard' }).click()

    // Form state should be cleared
    await waitFor(() => {
      expect(Object.keys(useFormState().formState.value).length).toBe(0)
    })
  })

  test('renders finished button when on last step and form is saved', async () => {
    // Pre-populate form state
    useFormState().formState.value = {
      hardwareType: 'metal',
    }

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
    useFormState().formState.value = {
      hardwareType: 'metal',
    }

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
