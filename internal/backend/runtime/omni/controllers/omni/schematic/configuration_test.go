// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package schematic_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/image-factory/pkg/schematic"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/imageropts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	schematicctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/schematic"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

//nolint:maintidx
func TestSchematicConfigurationReconcile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()

	factory := &mockImageFactoryClient{}

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(schematicctrl.NewConfigurationController(factory)))
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			r := require.New(t)

			machineName := "machine1"
			clusterName := "cluster"
			machineSet := "machineset"

			const talosVersion = "1.7.0"

			cluster := omni.NewCluster(clusterName)
			cluster.TypedSpec().Value.TalosVersion = talosVersion

			r.NoError(st.Create(ctx, cluster))

			machineStatus := omni.NewMachineStatus(machineName)
			machineStatus.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")

			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/hello-world-service
			expectedSchematic := "cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f"

			emptyRaw, err := (&schematic.Schematic{}).Marshal()
			r.NoError(err)

			machineStatus.TypedSpec().Value.TalosVersion = talosVersion
			machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
				FullId:           "test-full-id",
				Raw:              string(emptyRaw),
				Extensions:       []string{"siderolabs/hello-world-service"},
				InitialSchematic: expectedSchematic,
				InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
					Extensions: []string{"siderolabs/hello-world-service"},
				},
			}
			machineStatus.TypedSpec().Value.InitialTalosVersion = talosVersion
			machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{
				BootedWithUki: true,
			}
			machineStatus.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
				Platform: talosconstants.PlatformMetal,
			}

			clusterMachine := omni.NewClusterMachine(machineName)
			clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

			r.NoError(st.Create(ctx, machineStatus))

			// a schematic should already be created with the current list of extensions, without requiring a cluster machine
			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
					assertion.False(hasClusterLabel)

					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			r.NoError(st.Create(ctx, clusterMachine))

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
					assertion.True(hasClusterLabel)

					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// set empty extensions list for the cluster
			extensionsConfiguration := omni.NewExtensionsConfiguration("test")
			extensionsConfiguration.TypedSpec().Value.Extensions = nil
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

			r.NoError(st.Create(ctx, extensionsConfiguration))

			// customization: {}
			expectedSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// override extensions list for the machine set
			extensionsConfiguration = omni.NewExtensionsConfiguration("machineset")
			extensionsConfiguration.TypedSpec().Value.Extensions = []string{
				"siderolabs/something",
			}
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

			r.NoError(st.Create(ctx, extensionsConfiguration))

			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/something
			expectedSchematic = "df7c842f133b05c875f2139ea94b09eae3d425e00a95e6f9f54552f442d9f8c0"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// set overlay on the machine status by writing the raw YAML. The controller sources overlay from Raw.
			overlayRaw, err := (&schematic.Schematic{
				Overlay: schematic.Overlay{
					Name:  "rpi_generic",
					Image: "something",
				},
			}).Marshal()
			r.NoError(err)

			_, err = safe.StateUpdateWithConflicts(ctx, st, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Schematic.Raw = string(overlayRaw)

				return nil
			})

			r.NoError(err)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/something
			expectedSchematic = "f6a68c47512b4f3c50ccbd6d57873d2194dcac15f3a79d7703c05538a83429d7"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// override schematics on the machine level
			extensionsConfiguration = omni.NewExtensionsConfiguration("zzzz")
			extensionsConfiguration.TypedSpec().Value.Extensions = []string{
				"siderolabs/something-else",
			}
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelClusterMachine, machineName)

			r.NoError(st.Create(ctx, extensionsConfiguration))

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/something-else
			expectedSchematic = "d7eb0c567b0b108e9b69ee0217c0fed99847175549b48d7b41ec6ef45d993965"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// update extensions
			_, err = safe.StateUpdateWithConflicts(ctx, st, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
				res.TypedSpec().Value.Extensions = nil

				return nil
			})

			r.NoError(err)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization: {}
			expectedSchematic = "2611e4c1b6b8de906c9ad8f2248145d034ce8f657706407fe2f6a01086331a7d"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// reset everything to the default state, should revert back to the initial set of extensions

			rtestutils.DestroyAll[*omni.ExtensionsConfiguration](ctx, t, st)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/something
			expectedSchematic = "8ac31bbb181769d0963b217bb48f92839059ce90bc9e8b08836892c0182f8cb8"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			_, err = safe.StateUpdateWithConflicts(ctx, st, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.InitialTalosVersion = "1.5.0"

				return nil
			})

			r.NoError(err)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/bnx2-bnx2x
			//       - siderolabs/intel-ice-firmware
			expectedSchematic = "35a502528a50b5c9d264a152545c4b02c2b82a2a5c8fd7398baa9fe78abfb1a2"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// set empty extensions list for the cluster, should keep the old schematic ID
			extensionsConfiguration.TypedSpec().Value.Extensions = []string{}
			extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

			r.NoError(st.Create(ctx, extensionsConfiguration))

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// update extensions, should be still no-op as it's duplicate to what's selected by Omni
			_, err = safe.StateUpdateWithConflicts(ctx, st, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
				res.TypedSpec().Value.Extensions = []string{"siderolabs/bnx2-bnx2x"}

				return nil
			})

			r.NoError(err)

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// add an extra extension, schematic ID should change
			_, err = safe.StateUpdateWithConflicts(ctx, st, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
				res.TypedSpec().Value.Extensions = []string{
					"siderolabs/bnx2-bnx2x",
					"siderolabs/x11",
				}

				return nil
			})

			r.NoError(err)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/bnx2-bnx2x
			//       - siderolabs/intel-ice-firmware
			//       - siderolabs/x11
			expectedSchematic = "5fd4ef8a66795a9aba2520a2be1bb4fb64ef7405a775e40965cf6d7aa417665f"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// create kernel args, should change the schematic ID

			kernelArgs := omni.NewKernelArgs(machineName)
			kernelArgs.TypedSpec().Value.Args = []string{"foo=bar", "baz=qux"}

			r.NoError(st.Create(ctx, kernelArgs))

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   extraKernelArgs:
			//     - foo=bar
			//     - baz=qux
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/bnx2-bnx2x
			//       - siderolabs/intel-ice-firmware
			//       - siderolabs/x11
			expectedSchematic = "17b419c0d747bbd2399e2d06d16def170636569e9116e3e015b5be0015dd82c7"

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
				},
			)

			// set the UKI to false, the schematic should no more contain the kernel args (as updating them is not supported)

			_, err = safe.StateUpdateWithConflicts(ctx, st, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.SecurityState.BootedWithUki = false

				return nil
			})
			r.NoError(err)

			// overlay:
			//   image: something
			//   name: rpi_generic
			// customization:
			//   systemExtensions:
			//     officialExtensions:
			//       - siderolabs/bnx2-bnx2x
			//       - siderolabs/intel-ice-firmware
			//       - siderolabs/x11
			expectedSchematic = "5fd4ef8a66795a9aba2520a2be1bb4fb64ef7405a775e40965cf6d7aa417665f"

			rtestutils.AssertResources(ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
				assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
			})

			// update the MachineStatus to simulate an actual change of the schematic (e.g., the schematic change caused an upgrade)
			_, err = safe.StateUpdateWithConflicts(ctx, st, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Schematic.Extensions = []string{
					"siderolabs/bnx2-bnx2x",
					"siderolabs/intel-ice-firmware",
					"siderolabs/x11",
				}

				return nil
			})
			r.NoError(err)

			// destroy the ClusterMachine
			rtestutils.Destroy[*omni.ClusterMachine](ctx, t, st, []string{clusterMachine.Metadata().ID()})

			rtestutils.AssertResources(ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
				_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
				assertion.False(hasClusterLabel)
			})

			// Change the extensions in the ExtensionsConfiguration: because the machine is no more allocated, it should be no-op, and the existing list of extensions should be preserved.
			_, err = safe.StateUpdateWithConflicts(ctx, st, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
				res.TypedSpec().Value.Extensions = []string{
					"siderolabs/yet-another-extension",
				}

				return nil
			})
			r.NoError(err)

			rtestutils.AssertResources(ctx, t, st, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
				_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
				assertion.False(hasClusterLabel)

				assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
			})
		},
	)
}

// TestSchematicConfigurationPreservesRawFields verifies that the controller preserves all schematic fields
// that Omni does not typed-manage (overlay options, embedded machine config, bootloader, secureboot,
// disk image options) when it patches the machine's raw schematic and uploads the result to the factory.
//
// It also verifies that the controller uses the schematic id returned by the factory as the source of truth.
// The fake factory is configured with an Owner, which it stamps onto every upload (mirroring the Enterprise
// factory). The controller-published SchematicId must therefore match the id of the owner-tagged schematic,
// not the id of the un-tagged patched schematic the controller computed locally. We verify this by looking
// up the published id in the factory's storage and asserting the stored body carries the injected owner.
func TestSchematicConfigurationPreservesRawFields(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	const factoryOwner = "customer-acme"

	factory := &mockImageFactoryClient{Owner: factoryOwner}

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(schematicctrl.NewConfigurationController(factory)))
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			r := require.New(t)

			const (
				machineName  = "preserve-machine"
				clusterName  = "preserve-cluster"
				talosVersion = "1.10.0"
			)

			// Raw schematic carrying every field the controller does NOT typed-manage:
			// overlay incl. options, embeddedMachineConfiguration, bootloader, secureboot, diskImage.
			// Owner is left empty here intentionally: it is the factory's job to stamp it, and the
			// fake factory above is configured to do so. The extensions list is the only field the
			// controller will rewrite (from the machine's view), so we feed it a value that matches
			// what Omni will compute (no cluster customization) to keep the extensions assertion
			// independent from the preservation assertions.
			rawSchematic := schematic.Schematic{
				Overlay: schematic.Overlay{
					Name:  "rpi_generic",
					Image: "factory.example.com/overlays/rpi_generic:v1.2.3",
					Options: map[string]any{
						"deviceTreeOverlays": []any{"some-overlay"},
					},
				},
				Customization: schematic.Customization{
					EmbeddedMachineConfiguration: "version: v1alpha1\nmachine: {}\n",
					ExtraKernelArgs:              []string{"console=ttyS0"},
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{"siderolabs/hello-world-service"},
					},
					Bootloader: imageropts.BootLoaderKindGrub,
					SecureBoot: schematic.SecureBootCustomization{
						IncludeWellKnownCertificates: true,
					},
					DiskImage: schematic.DiskImageCustomization{
						SectorSize: 4096,
					},
				},
			}

			rawYAML, err := rawSchematic.Marshal()
			r.NoError(err)

			rawSchematicID, err := rawSchematic.ID()
			r.NoError(err)

			cluster := omni.NewCluster(clusterName)
			cluster.TypedSpec().Value.TalosVersion = talosVersion
			r.NoError(st.Create(ctx, cluster))

			machineStatus := omni.NewMachineStatus(machineName)
			machineStatus.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
			machineStatus.TypedSpec().Value.TalosVersion = talosVersion
			machineStatus.TypedSpec().Value.InitialTalosVersion = talosVersion
			machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
				FullId:           rawSchematicID,
				Raw:              string(rawYAML),
				Extensions:       []string{"siderolabs/hello-world-service"},
				KernelArgs:       []string{"console=ttyS0"},
				InitialSchematic: rawSchematicID,
				InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
					Extensions: []string{"siderolabs/hello-world-service"},
				},
			}
			machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{
				BootedWithUki: false,
			}
			machineStatus.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
				Platform: talosconstants.PlatformMetal,
			}
			r.NoError(st.Create(ctx, machineStatus))

			// Capture the SchematicId the controller publishes, then look up what it actually
			// uploaded to the factory. The id is not pre-computed because PatchSchematic's
			// canonical re-marshal can produce a different id than rawSchematic.ID() if the
			// extensions/kernel-args differ from what Omni computes.
			var publishedID string

			rtestutils.AssertResources(
				ctx, t, st, []string{machineName},
				func(sc *omni.SchematicConfiguration, assertion *assert.Assertions) {
					assertion.NotEmpty(sc.TypedSpec().Value.SchematicId)

					publishedID = sc.TypedSpec().Value.SchematicId
				},
			)

			// If the controller computed the SchematicId locally (off the patched-but-not-owned schematic)
			// instead of trusting the factory's response, publishedID would not match the factory's
			// owner-tagged storage key and this lookup would miss.
			stored, ok := factory.get(publishedID)
			r.True(ok, "controller-published schematic %q was not uploaded to the factory", publishedID)

			// Factory-injected owner must come through on the stored body.
			assert.Equal(t, factoryOwner, stored.Owner, "Owner injected by the factory was lost")

			// Fields the controller does not typed-manage must round-trip unchanged.
			assert.Equal(t, "rpi_generic", stored.Overlay.Name, "Overlay.Name dropped")
			assert.Equal(t, "factory.example.com/overlays/rpi_generic:v1.2.3", stored.Overlay.Image, "Overlay.Image dropped")
			assert.Equal(t, rawSchematic.Overlay.Options, stored.Overlay.Options, "Overlay.Options dropped")
			assert.Equal(t, rawSchematic.Customization.EmbeddedMachineConfiguration, stored.Customization.EmbeddedMachineConfiguration, "EmbeddedMachineConfiguration dropped")
			assert.Equal(t, imageropts.BootLoaderKindGrub, stored.Customization.Bootloader, "Bootloader dropped")
			assert.True(t, stored.Customization.SecureBoot.IncludeWellKnownCertificates, "SecureBoot.IncludeWellKnownCertificates dropped")
			assert.Equal(t, uint(4096), stored.Customization.DiskImage.SectorSize, "DiskImage.SectorSize dropped")

			// Sanity: extensions and kernel args (the fields the controller does manage) come out as expected.
			assert.Equal(t, []string{"siderolabs/hello-world-service"}, stored.Customization.SystemExtensions.OfficialExtensions)
			assert.Equal(t, []string{"console=ttyS0"}, stored.Customization.ExtraKernelArgs)
		},
	)
}

// mockImageFactoryClient is an in-process fake of the image factory client interface.
// It computes the canonical id of every schematic it receives and stores it for later
// inspection by tests, so tests can verify what the controller actually uploaded
// without going through HTTP serialization round-trip.
//
// If Owner is set, every submitted schematic is tagged with that owner before its id
// is computed and stored. This mirrors the Enterprise factory's behavior of stamping
// the customer's owner onto every upload, and lets tests verify that the controller
// uses the id returned by the factory as the source of truth (not a locally computed one).
type mockImageFactoryClient struct {
	schematics map[string]schematic.Schematic
	Owner      string
	mu         sync.Mutex
}

func (m *mockImageFactoryClient) EnsureSchematic(_ context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error) {
	if m.Owner != "" {
		inputSchematic.Owner = m.Owner
	}

	id, err := inputSchematic.ID()
	if err != nil {
		return "", nil, err
	}

	stored := inputSchematic

	m.mu.Lock()
	if m.schematics == nil {
		m.schematics = map[string]schematic.Schematic{}
	}

	m.schematics[id] = stored
	m.mu.Unlock()

	return id, &stored, nil
}

// get is a test helper for reading back what the controller uploaded.
func (m *mockImageFactoryClient) get(id string) (schematic.Schematic, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.schematics[id]

	return s, ok
}
