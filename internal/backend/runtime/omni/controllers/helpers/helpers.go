// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package helpers contains common utility methods for COSI controllers of Omni.
package helpers

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"iter"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// InputResourceVersionAnnotation is the annotation name where the inputs version sha is stored.
const InputResourceVersionAnnotation = "inputResourceVersion"

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

	version, found := out.Metadata().Annotations().Get(InputResourceVersionAnnotation)

	if found && version == inVersion {
		return false
	}

	out.Metadata().Annotations().Set(InputResourceVersionAnnotation, inVersion)

	return true
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

// SyncAllLabels synchronizes all labels from one resource to another.
// It copies all labels from the source resource to the destination resource
// and removes any labels from the destination resource that are not present in the source resource.
func SyncAllLabels(src, dst resource.Resource) {
	dst.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for key, value := range src.Metadata().Labels().Raw() {
			tmp.Set(key, value)
		}

		for key := range dst.Metadata().Labels().Raw() {
			if _, ok := src.Metadata().Labels().Get(key); !ok {
				tmp.Delete(key)
			}
		}
	})
}

// SyncLabels synchronizes the specified labels from one resource to another.
// It copies the specified labels from the source resource to the destination resource
// and removes any of those labels from the destination resource that are not present in the source resource.
func SyncLabels(src, dst resource.Resource, keys ...string) {
	dst.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
		for _, key := range keys {
			if label, ok := src.Metadata().Labels().Get(key); ok {
				tmp.Set(key, label)
			}
		}

		for _, key := range keys {
			if _, ok := src.Metadata().Labels().Get(key); !ok {
				tmp.Delete(key)
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

// SyncAllAnnotations synchronizes all annotations from one resource to another.
// It copies all annotations from the source resource to the destination resource
// and removes any annotations from the destination resource that are not present in the source resource.
func SyncAllAnnotations(src, dst resource.Resource) {
	dst.Metadata().Annotations().Do(func(tmp kvutils.TempKV) {
		for key, value := range src.Metadata().Annotations().Raw() {
			tmp.Set(key, value)
		}

		for key := range dst.Metadata().Annotations().Raw() {
			if _, ok := src.Metadata().Annotations().Get(key); !ok {
				tmp.Delete(key)
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

// GetTalosClient for the machine id.
// Automatically pick secure or insecure client.
func GetTalosClient[T interface {
	*V
	generic.ResourceWithRD
}, V any](ctx context.Context, r controller.Reader, address string, machineResource T) (*client.Client, error) {
	opts := talos.GetSocketOptions(address)

	createInsecureClient := func() (*client.Client, error) {
		return client.New(ctx,
			append(
				opts,
				client.WithTLSConfig(&tls.Config{
					InsecureSkipVerify: true,
				}),
				client.WithEndpoints(address),
			)...)
	}

	if machineResource == nil {
		return createInsecureClient()
	}

	machineStatusSnapshot, err := safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r, machineResource.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if machineStatusSnapshot != nil && machineStatusSnapshot.TypedSpec().Value.MachineStatus.Stage == machine.MachineStatusEvent_MAINTENANCE {
		return createInsecureClient()
	}

	clusterName, ok := machineResource.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return createInsecureClient()
	}

	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(clusterName).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("cluster '%s' failed to get talosconfig: %w", clusterName, err)
	}

	if talosConfig == nil {
		return createInsecureClient()
	}

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	config := omni.NewTalosClientConfig(talosConfig, endpoints...)
	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", machineResource.Metadata().ID(), err)
	}

	return result, nil
}

// TeardownAndDestroy calls Teardown for a resource, then calls Destroy if the resource doesn't have finalizers.
// It returns true if the resource were destroyed.
func TeardownAndDestroy(
	ctx context.Context,
	r controller.Writer,
	ptr resource.Pointer,
	options ...controller.DeleteOption,
) (bool, error) {
	return helpers.TeardownAndDestroy(ctx, r, ptr, options...)
}

// TeardownAndDestroyAll calls Teardown for all resources, then calls Destroy for all resources which
// have no finalizers.
// It returns true if all resources were destroyed.
func TeardownAndDestroyAll(
	ctx context.Context,
	r controller.Writer,
	resources iter.Seq[resource.Pointer],
	options ...controller.DeleteOption,
) (bool, error) {
	return helpers.TeardownAndDestroyAll(ctx, r, resources, options...)
}
