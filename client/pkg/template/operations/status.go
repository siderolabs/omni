// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/xlab/treeprint"
	"golang.org/x/term"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/template/operations/internal/statustree"
)

// StatusOptions configures the status operation.
type StatusOptions struct {
	Wait  bool
	Quiet bool
}

// StatusTemplate queries, renders and (optionally) waits for the cluster status (health).
func StatusTemplate(ctx context.Context, templateReader io.Reader, out io.Writer, st state.State, options StatusOptions) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	if err = tmpl.Validate(); err != nil {
		return err
	}

	return statusTemplate(ctx, tmpl, out, st, options)
}

// StatusCluster queries, renders and (optionally) waits for the cluster status (health).
func StatusCluster(ctx context.Context, clusterName string, out io.Writer, st state.State, options StatusOptions) error {
	tmpl := template.WithCluster(clusterName)

	return statusTemplate(ctx, tmpl, out, st, options)
}

//nolint:gocognit,gocyclo,cyclop
func statusTemplate(ctx context.Context, tmpl *template.Template, out io.Writer, st state.State, options StatusOptions) error {
	clusterName, err := tmpl.ClusterName()
	if err != nil {
		return err
	}

	// initiate a watch on all resources which are part of the cluster status
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	if err = st.Watch(ctx, omni.NewClusterStatus(resources.DefaultNamespace, clusterName).Metadata(), watchCh); err != nil {
		return err
	}

	resourceTypes := []resource.Type{
		omni.MachineSetStatusType,
		omni.ControlPlaneStatusType,
		omni.LoadBalancerStatusType,
		omni.KubernetesUpgradeStatusType,
		omni.TalosUpgradeStatusType,
		omni.ClusterMachineStatusType,
	}

	for _, resourceType := range resourceTypes {
		if err = st.WatchKind(
			ctx,
			resource.NewMetadata(resources.DefaultNamespace, resourceType, "", resource.VersionUndefined),
			watchCh,
			state.WithBootstrapContents(true),
			state.WatchWithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		); err != nil {
			return err
		}
	}

	resources := map[string]resource.Resource{}
	pendingBootstraps := len(resourceTypes)

	var (
		clusterBootstrapped, startedRendering, hasUpdates bool
		prevLines                                         []byte
	)

	renderTicker := time.NewTicker(time.Second)
	defer renderTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-renderTicker.C:
			if !hasUpdates {
				continue
			}

			hasUpdates = false

			if options.Quiet {
				continue
			}

			newLines, healthy := render(resources)

			if err = printStatus(out, prevLines, newLines); err != nil {
				return err
			}

			prevLines = newLines

			if healthy {
				// done waiting
				return nil
			}
		case event := <-watchCh:
			hasUpdates = true

			switch event.Type {
			case state.Errored:
				return fmt.Errorf("watch failed: %w", event.Error)
			case state.Bootstrapped:
				pendingBootstraps--
			case state.Created, state.Updated:
				if event.Resource.Metadata().Type() == omni.ClusterStatusType {
					clusterBootstrapped = true
				}

				resources[resource.String(event.Resource)] = event.Resource
			case state.Destroyed:
				delete(resources, resource.String(event.Resource))
			}

			// render the initial state once fully bootstrapped
			if !startedRendering && clusterBootstrapped && pendingBootstraps == 0 {
				startedRendering = true
			} else {
				continue
			}

			newLines, healthy := render(resources)

			if !options.Quiet {
				if err = printStatus(out, prevLines, newLines); err != nil {
					return err
				}

				prevLines = newLines
			}

			hasUpdates = false

			if healthy {
				return nil
			}

			if !options.Wait {
				return fmt.Errorf("cluster is not healthy")
			}
		}
	}
}

func expandTree(tree treeprint.Tree, item statustree.NodeWrapper, resources map[string]resource.Resource) {
	var nextLevel []statustree.NodeWrapper

	for _, r := range resources {
		if item.IsParentOf(r) {
			nextLevel = append(nextLevel, statustree.NodeWrapper{Resource: r})
		}
	}

	slices.SortFunc(nextLevel, statustree.NodeWrapper.Compare)

	for _, item := range nextLevel {
		subtree := tree.AddBranch(item)
		expandTree(subtree, item, resources)
	}
}

// render builds a tree of resources and renders it to the buffer.
func render(resources map[string]resource.Resource) ([]byte, bool) {
	var clusterStatus *omni.ClusterStatus

	healthy := true

	for _, r := range resources {
		switch item := r.(type) {
		case *omni.ClusterStatus:
			clusterStatus = item

			healthy = healthy && item.TypedSpec().Value.Phase == specs.ClusterStatusSpec_RUNNING && item.TypedSpec().Value.Ready
		case *omni.MachineSetStatus:
			healthy = healthy && item.TypedSpec().Value.Phase == specs.MachineSetPhase_Running && item.TypedSpec().Value.Ready
		case *omni.KubernetesUpgradeStatus:
			healthy = healthy && item.TypedSpec().Value.Phase == specs.KubernetesUpgradeStatusSpec_Done
		case *omni.TalosUpgradeStatus:
			healthy = healthy && item.TypedSpec().Value.Phase == specs.TalosUpgradeStatusSpec_Done
		}
	}

	if clusterStatus == nil {
		return nil, false
	}

	root := statustree.NodeWrapper{Resource: clusterStatus}
	tree := treeprint.NewWithRoot(root)

	expandTree(tree, root, resources)

	return tree.Bytes(), healthy
}

// printStatus prints the tree to the terminal.
//
// If terminal supports it, previous tree is erased.
func printStatus(out io.Writer, prevLines, newLines []byte) error {
	var (
		w          int
		isTerminal bool
	)

	if f, ok := out.(*os.File); ok {
		w, _, _ = term.GetSize(int(f.Fd())) //nolint:errcheck
		isTerminal = term.IsTerminal(int(f.Fd()))
	}

	if w <= 0 {
		w = 80
	}

	if isTerminal {
		for _, outputLine := range strings.Split(string(prevLines), "\n") {
			for range (utf8.RuneCountInString(outputLine) + w - 1) / w {
				fmt.Fprint(out, "\033[A\033[K") // cursor up, clear line
			}
		}
	}

	_, err := out.Write(newLines)

	return err
}
