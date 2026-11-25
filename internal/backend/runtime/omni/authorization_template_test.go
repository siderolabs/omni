// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/authz/authztest"
)

const (
	templateControlPlane = "4b9797ae-55d7-4717-a19e-b322a2d83d39"
	templateWorker1      = "947f7a99-0f46-4a9e-ab02-205f4e1b7009"
	templateWorker2      = "b7201377-6194-4796-a831-295e865668e2"
	templateGPUWorker    = "6d8f28c6-2358-4503-936a-723508112185"
	templateOtherMachine = "03595977-1864-4e8a-b917-b40a6080de2f"
	templateNewMachine   = "424c3bbe-1ec2-475e-b804-abf77d6b6c30"
)

// clusterTemplate is the content of a cluster template, with only the parts which the test changes.
type clusterTemplate struct {
	installDisks map[string]string // machine ID to disk
	kernelArgs   map[string]string // machine ID to kernel arg
	cluster      string
	kubernetes   string
	talos        string
	maxMapCount  string // sysctl in the config patch of the cluster, no patch when empty
	workerLabel  string // node label in the config patch of the workers, no patch when empty
	controlPlane string
	workers      []string
	gpuWorkers   []string // machines of a second worker machine set
	locked       bool
}

func (tmpl clusterTemplate) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "kind: Cluster\nname: %s\nkubernetes:\n  version: %s\ntalos:\n  version: %s\n", tmpl.cluster, tmpl.kubernetes, tmpl.talos)

	if tmpl.locked {
		sb.WriteString("locked: true\n")
	}

	if tmpl.maxMapCount != "" {
		fmt.Fprintf(&sb, "patches:\n  - name: sysctls\n    inline:\n      machine:\n        sysctls:\n          vm.max_map_count: %q\n", tmpl.maxMapCount)
	}

	fmt.Fprintf(&sb, "---\nkind: ControlPlane\nmachines:\n  - %s\n", tmpl.controlPlane)

	machineSet := func(name string, machines []string, label string) {
		if len(machines) == 0 {
			return
		}

		sb.WriteString("---\nkind: Workers\n")

		if name != "" {
			fmt.Fprintf(&sb, "name: %s\n", name)
		}

		sb.WriteString("machines:\n")

		for _, machine := range machines {
			fmt.Fprintf(&sb, "  - %s\n", machine)
		}

		if label != "" {
			fmt.Fprintf(&sb, "patches:\n  - name: node-labels\n    inline:\n      machine:\n        nodeLabels:\n          pool: %s\n", label)
		}
	}

	machineSet("", tmpl.workers, tmpl.workerLabel)
	machineSet("gpu", tmpl.gpuWorkers, "")

	for _, machine := range slices.Concat([]string{tmpl.controlPlane}, tmpl.workers, tmpl.gpuWorkers) {
		disk, kernelArg := tmpl.installDisks[machine], tmpl.kernelArgs[machine]
		if disk == "" && kernelArg == "" {
			continue
		}

		fmt.Fprintf(&sb, "---\nkind: Machine\nname: %s\n", machine)

		if disk != "" {
			fmt.Fprintf(&sb, "install:\n  disk: %s\n", disk)
		}

		if kernelArg != "" {
			fmt.Fprintf(&sb, "kernelArgs:\n  - %s\n", kernelArg)
		}
	}

	return sb.String()
}

// TestTemplateOperationsWithClusterAccess runs the cluster template operations of omnictl as a user who has access only through
// the access policy, so each read, write and watch they do must be limited to the cluster of the template.
//
// The template is synced many times with a change in between, the way a cluster is managed over its life.
// All validations of the resource API run, as in production. Omni's controllers do not run.
func TestTemplateOperationsWithClusterAccess(t *testing.T) {
	for _, test := range []struct {
		name    string
		subject string
		preset  role.Role // the preset (role) of the user, next to what the access policy gives
		cluster string
	}{
		{name: "operator of some clusters", subject: "acl-operator-a", preset: role.None, cluster: "template-a"},
		{name: "operator of some clusters, with the Reader preset", subject: "acl-operator-a", preset: role.Reader, cluster: "template-a"},
		{name: "operator of every cluster", subject: "acl-operator-all", preset: role.None, cluster: "template"},
	} {
		t.Run(test.name, func(t *testing.T) {
			keys := authztest.GenerateKeys(t)
			w := newValidatedWorld(t, keys)
			internalCtx := actor.MarkContextAsInternalActor(t.Context())

			_, err := safe.StateUpdateWithConflicts(internalCtx, w.unauthorized, authres.NewUser(test.subject).Metadata(), func(user *authres.User) error {
				user.TypedSpec().Value.Role = string(test.preset)

				return nil
			})
			require.NoError(t, err)

			ctx := w.subjectContext(t, test.subject)

			sync := func(ctx context.Context, tmpl clusterTemplate) error {
				return operations.SyncTemplate(ctx, strings.NewReader(tmpl.String()), io.Discard, w.authorized, operations.SyncOptions{}, nil)
			}

			// What the user has no access to: a cluster outside of the access policy rule, and the settings of a machine in no cluster.
			const otherCluster = "other-b"

			w.createTemplateFixtures(internalCtx, t, test.cluster)
			require.NoError(t, sync(internalCtx, clusterTemplate{cluster: otherCluster, kubernetes: "v1.33.0", talos: "v1.10.0", controlPlane: templateOtherMachine}))

			tmpl := clusterTemplate{cluster: test.cluster, kubernetes: "v1.33.0", talos: "v1.10.0", controlPlane: templateControlPlane}

			for _, step := range []struct {
				change func(*clusterTemplate)
				name   string
			}{
				{name: "create the cluster with a control plane", change: func(*clusterTemplate) {}},
				{name: "sync again without a change", change: func(*clusterTemplate) {}},
				{name: "add a config patch to the cluster", change: func(tmpl *clusterTemplate) { tmpl.maxMapCount = "262144" }},
				{name: "add workers with a config patch", change: func(tmpl *clusterTemplate) {
					tmpl.workers = []string{templateWorker1}
					tmpl.workerLabel = "general"
				}},
				{name: "set the install disk of a worker", change: func(tmpl *clusterTemplate) { tmpl.installDisks = map[string]string{templateWorker1: "/dev/vda"} }},
				{name: "add a second worker", change: func(tmpl *clusterTemplate) { tmpl.workers = []string{templateWorker1, templateWorker2} }},
				{name: "change the config patch of the cluster", change: func(tmpl *clusterTemplate) { tmpl.maxMapCount = "524288" }},
				{name: "upgrade Kubernetes", change: func(tmpl *clusterTemplate) { tmpl.kubernetes = "v1.33.1" }},
				{name: "upgrade Talos", change: func(tmpl *clusterTemplate) { tmpl.talos = "v1.10.1" }},
				{name: "add a second worker machine set", change: func(tmpl *clusterTemplate) { tmpl.gpuWorkers = []string{templateGPUWorker} }},
				{name: "change the install disk of a worker", change: func(tmpl *clusterTemplate) { tmpl.installDisks = map[string]string{templateWorker1: "/dev/sda"} }},
				{name: "lock the cluster", change: func(tmpl *clusterTemplate) { tmpl.locked = true }},
				{name: "unlock the cluster", change: func(tmpl *clusterTemplate) { tmpl.locked = false }},
				{name: "remove a worker", change: func(tmpl *clusterTemplate) { tmpl.workers = []string{templateWorker1} }},
				{name: "remove the install disk of a worker", change: func(tmpl *clusterTemplate) { tmpl.installDisks = nil }},
				{name: "remove the config patch of the workers", change: func(tmpl *clusterTemplate) { tmpl.workerLabel = "" }},
				{name: "remove the second worker machine set", change: func(tmpl *clusterTemplate) { tmpl.gpuWorkers = nil }},
			} {
				step.change(&tmpl)

				require.NoError(t, operations.DiffTemplate(ctx, strings.NewReader(tmpl.String()), io.Discard, w.authorized, nil), "diff: %s", step.name)
				require.NoError(t, sync(ctx, tmpl), step.name)
			}

			// The sync leaves the nodes of a removed machine set to Omni's controller, which does not run here.
			require.NoError(t, w.unauthorized.Destroy(internalCtx, omni.NewMachineSetNode(templateGPUWorker, omni.NewMachineSet("")).Metadata()))

			var exported bytes.Buffer

			_, err = operations.ExportTemplate(ctx, w.authorized, test.cluster, false, &exported)
			require.NoError(t, err, "export the cluster")
			assert.Contains(t, exported.String(), "vm.max_map_count")

			// the status is "not healthy" here, as nothing reports the health of the cluster
			err = operations.StatusTemplate(ctx, strings.NewReader(tmpl.String()), io.Discard, w.authorized, operations.StatusOptions{Quiet: true}, nil)
			require.EqualError(t, err, "cluster is not healthy", "status of the cluster")

			// The user cannot write kernel args, as they are a setting of a machine. The sync must fail before it adds the machine to the cluster.
			withKernelArgs := tmpl
			withKernelArgs.workers = []string{templateWorker1, templateNewMachine}
			withKernelArgs.kernelArgs = map[string]string{templateNewMachine: "console=ttyS0"}

			err = sync(ctx, withKernelArgs)
			assert.Equal(t, codes.PermissionDenied, status.Code(err), "sync kernel args: %v", err)

			// A user who cannot read kernel args is stopped before the machine is added to the cluster. A Reader can read them,
			// so the sync only fails at the write, after the machine was added.
			_, err = safe.StateGetByID[*omni.MachineSetNode](internalCtx, w.unauthorized, templateNewMachine)
			if test.preset == role.None {
				assert.True(t, state.IsNotFoundError(err), "the machine is not added to the cluster: %v", err)
			} else {
				require.NoError(t, w.unauthorized.Destroy(internalCtx, omni.NewMachineSetNode(templateNewMachine, omni.NewMachineSet("")).Metadata()))
			}

			// A machine of another cluster must stay in its cluster.
			withOtherMachine := tmpl
			withOtherMachine.workers = []string{templateWorker1, templateOtherMachine}

			require.Error(t, sync(ctx, withOtherMachine), "sync a machine of another cluster")

			otherNode, err := safe.StateGetByID[*omni.MachineSetNode](internalCtx, w.unauthorized, templateOtherMachine)
			require.NoError(t, err)

			nodeCluster, _ := otherNode.Metadata().Labels().Get(omni.LabelCluster)
			assert.Equal(t, otherCluster, nodeCluster)

			require.NoError(t, operations.DeleteTemplate(ctx, strings.NewReader(tmpl.String()), io.Discard, w.authorized, operations.SyncOptions{}, nil), "delete the cluster")

			_, err = safe.StateGetByID[*omni.Cluster](internalCtx, w.unauthorized, test.cluster)
			assert.True(t, state.IsNotFoundError(err), "the cluster is destroyed: %v", err)

			// Kernel args which someone with access to machines has set through a template stay as they are, and stay marked
			// as managed by cluster templates: the user cannot remove the mark, and that does not fail the sync or the delete.
			kernelArgs, err := safe.StateGetByID[*omni.KernelArgs](internalCtx, w.unauthorized, templateControlPlane)
			require.NoError(t, err)
			assert.Equal(t, []string{"console=ttyS0"}, kernelArgs.TypedSpec().Value.Args)

			_, managed := kernelArgs.Metadata().Annotations().Get(omni.ResourceManagedByClusterTemplates)
			assert.True(t, managed, "the kernel args are not changed")

			if test.subject == "acl-operator-all" {
				return // has access to every cluster
			}

			err = sync(ctx, clusterTemplate{cluster: otherCluster, kubernetes: "v1.33.0", talos: "v1.10.0", controlPlane: templateOtherMachine, maxMapCount: "262144"})
			assert.Equal(t, codes.PermissionDenied, status.Code(err), "sync a cluster outside of the access policy rule: %v", err)
		})
	}
}

// createTemplateFixtures creates what the template operations need and a user with access to a cluster cannot create:
// the Talos versions, and what Omni's controllers would create for the cluster.
func (w *world) createTemplateFixtures(ctx context.Context, t *testing.T, cluster string) {
	// a config patch of a machine in no cluster
	machinePatch := omni.NewConfigPatch("of-free-machine")
	machinePatch.Metadata().Labels().Set(omni.LabelMachine, authztest.FreeMachine)

	// the control plane machine as a member of the cluster
	clusterMachine := omni.NewClusterMachine(templateControlPlane)
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, cluster)

	// kernel args which someone with access to machines has set
	kernelArgs := omni.NewKernelArgs(templateControlPlane)
	kernelArgs.Metadata().Annotations().Set(omni.ResourceManagedByClusterTemplates, "")
	kernelArgs.TypedSpec().Value.Args = []string{"console=ttyS0"}

	talosVersions := []string{"1.10.0", "1.10.1"}

	fixtures := make([]resource.Resource, 0, 4+len(talosVersions))
	fixtures = append(fixtures, omni.NewClusterStatus(cluster), machinePatch, clusterMachine, kernelArgs)

	for _, version := range talosVersions {
		talosVersion := omni.NewTalosVersion(version)
		talosVersion.TypedSpec().Value.Version = version
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.33.0", "1.33.1"}

		fixtures = append(fixtures, talosVersion)
	}

	for _, res := range fixtures {
		require.NoError(t, w.unauthorized.Create(ctx, res))
	}
}

// subjectContext signs in the subject with the given name.
func (w *world) subjectContext(t *testing.T, name string) context.Context {
	for _, s := range authztest.Subjects {
		if s.Name == name {
			return authztest.SignIn(t, w.unauthorized, w.keys, s)
		}
	}

	require.FailNow(t, "unknown subject", name)

	return nil
}

// newValidatedWorld is newWorld with all validations of the resource API on top of the access checks, as in production.
func newValidatedWorld(t *testing.T, keys authztest.Keys) *world {
	authorized, unauthorized, err := omniruntime.NewValidatedState("https://omni.example.org")
	require.NoError(t, err)

	authztest.Seed(t, unauthorized, keys)

	return &world{authorized: authorized, unauthorized: unauthorized, keys: keys}
}
