// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package omni contains controllers that are managing Omni resources: Machines, Clusters, etc..
package omni

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const inputResourceVersionAnnotation = "inputResourceVersion"

// UpdateInputsVersions generates a hash of the resource by combining its inputs.
func UpdateInputsVersions[T resource.Resource](out resource.Resource, inputs ...T) bool {
	return UpdateInputsAnnotation(out, xslices.Map(inputs, func(input T) string {
		return fmt.Sprintf("%s/%s@%s", input.Metadata().Type(), input.Metadata().ID(), input.Metadata().Version())
	})...)
}

// UpdateInputsAnnotation updates the annotation with the input resource version and returns if it has changed.
func UpdateInputsAnnotation(out resource.Resource, versions ...string) bool {
	hash := sha256.New()

	for i, version := range versions {
		if i > 0 {
			hash.Write([]byte(","))
		}

		hash.Write([]byte(version))
	}

	inVersion := hex.EncodeToString(hash.Sum(nil))

	version, found := out.Metadata().Annotations().Get(inputResourceVersionAnnotation)

	if found && version == inVersion {
		return false
	}

	out.Metadata().Annotations().Set(inputResourceVersionAnnotation, inVersion)

	return true
}

func trackResource(r controller.ReaderWriter, ns resource.Namespace, resourceType resource.Type, listOptions ...state.ListOption) *resourceTracker {
	return &resourceTracker{
		touched:     map[string]struct{}{},
		listMD:      resource.NewMetadata(ns, resourceType, "", resource.VersionUndefined),
		listOptions: listOptions,
		r:           r,
	}
}

type resourceTracker struct {
	owner                 string
	r                     controller.ReaderWriter
	beforeDestroyCallback func(resource.Resource) error
	touched               map[resource.ID]struct{}
	listOptions           []state.ListOption
	listMD                resource.Metadata
}

func (rt *resourceTracker) keep(res resource.Resource) {
	rt.touched[res.Metadata().ID()] = struct{}{}
}

func (rt *resourceTracker) getItemsToDestroy(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	list, err := rt.r.List(ctx, rt.listMD, rt.listOptions...)
	if err != nil {
		return nil, fmt.Errorf("error listing resources: %w", err)
	}

	for _, res := range list.Items {
		if _, ok := rt.touched[res.Metadata().ID()]; !ok {
			resources = append(resources, res)
		}
	}

	return resources, nil
}

func (rt *resourceTracker) cleanup(ctx context.Context) error {
	items, err := rt.getItemsToDestroy(ctx)
	if err != nil {
		return fmt.Errorf("error getting items to destroy: %w", err)
	}

	for _, res := range items {
		if rt.owner != "" && res.Metadata().Owner() != rt.owner {
			continue
		}

		var ready bool

		if ready, err = rt.r.Teardown(ctx, res.Metadata()); err != nil {
			return fmt.Errorf("error tearing down resource '%s': %w", res.Metadata().ID(), err)
		}

		if !ready {
			continue
		}

		if rt.beforeDestroyCallback != nil {
			if err = rt.beforeDestroyCallback(res); err != nil {
				return fmt.Errorf("error running before destroy callback for resource '%s': %w", res.Metadata().ID(), err)
			}
		}

		if err = rt.r.Destroy(ctx, res.Metadata()); err != nil {
			return fmt.Errorf("error destroying resource '%s': %w", res.Metadata().ID(), err)
		}
	}

	return nil
}

// BeforeDestroy register the callback which is called before destroying the resource.
func (rt *resourceTracker) BeforeDestroy(f func(res resource.Resource) error) {
	rt.beforeDestroyCallback = f
}

// CopyAllLabels copies all labels from one resource to another.
func CopyAllLabels(src, dst resource.Resource) {
	dst.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for key, value := range src.Metadata().Labels().Raw() {
			tmp.Set(key, value)
		}
	})
}

// CopyLabels copies the labels from one resource to another.
func CopyLabels(src, dst resource.Resource, keys ...string) {
	dst.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for _, key := range keys {
			if label, ok := src.Metadata().Labels().Get(key); ok {
				tmp.Set(key, label)
			}
		}
	})
}

// CopyAllAnnotations copies all annotations from one resource to another.
func CopyAllAnnotations(src, dst resource.Resource) {
	dst.Metadata().Annotations().Do(func(tmp kvutils.TempKV) {
		for key, value := range src.Metadata().Annotations().Raw() {
			tmp.Set(key, value)
		}
	})
}

// CopyAnnotations copies annotations from one resource to another.
func CopyAnnotations(src, dst resource.Resource, annotations ...string) {
	dst.Metadata().Annotations().Do(func(tmp kvutils.TempKV) {
		for _, key := range annotations {
			if label, ok := src.Metadata().Annotations().Get(key); ok {
				tmp.Set(key, label)
			}
		}
	})
}

// CopyUserLabels copies all user labels from one resource to another.
// It removes all user labels on the target that are not present in the source resource.
// System labels are not copied.
func CopyUserLabels(target resource.Resource, labels map[string]string) {
	ClearUserLabels(target)

	if len(labels) == 0 {
		return
	}

	target.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for key, value := range labels {
			if strings.HasPrefix(key, omni.SystemLabelPrefix) {
				continue
			}

			tmp.Set(key, value)
		}
	})
}

// ClearUserLabels removes all user labels from the resource.
func ClearUserLabels(res resource.Resource) {
	res.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for key := range res.Metadata().Labels().Raw() {
			if strings.HasPrefix(key, omni.SystemLabelPrefix) {
				continue
			}

			tmp.Delete(key)
		}
	})
}

// withFinalizerCheck wraps a [cleanup.Handler] with a check that needs to pass before the handler is called.
func withFinalizerCheck[Input generic.ResourceWithRD](handler cleanup.Handler[Input], check func(input Input) error) cleanup.Handler[Input] {
	return &cleanupChecker[Input]{
		next:  handler,
		check: check,
	}
}

type cleanupChecker[Input generic.ResourceWithRD] struct {
	next  cleanup.Handler[Input]
	check func(input Input) error
}

// FinalizerRemoval implements [cleanup.Handler].
// It first runs the check, and only if it passes, it calls the underlying handler's FinalizerRemoval.
func (c cleanupChecker[Input]) FinalizerRemoval(ctx context.Context, r controller.Runtime, logger *zap.Logger, input Input) error {
	if err := c.check(input); err != nil {
		logger.Warn("failed to cleanup outputs for resource", zap.Error(err), zap.String("resource_id", input.Metadata().ID()))

		return nil
	}

	if err := c.next.FinalizerRemoval(ctx, r, logger, input); err != nil {
		return err
	}

	return nil
}

// Inputs implements [cleanup.Handler].
func (c cleanupChecker[Input]) Inputs() []controller.Input {
	return c.next.Inputs()
}

// Outputs implements [cleanup.Handler].
func (c cleanupChecker[Input]) Outputs() []controller.Output {
	return c.next.Outputs()
}
