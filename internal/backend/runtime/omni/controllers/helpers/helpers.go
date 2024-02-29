// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package helpers contains common utility methods for COSI controllers of Omni.
package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
