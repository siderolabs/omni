// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func Benchmark_ForAllCompatibleVersions(b *testing.B) {
	b.StopTimer()

	talosVersions := []string{
		"1.3.0",
		"1.3.1",
		"1.3.2",
		"1.3.3",
		"1.3.4",
		"1.3.5",
		"1.3.6",
		"1.3.7",
		"1.4.0",
		"1.4.1",
		"1.4.4",
		"1.4.5",
		"1.4.6",
		"1.4.7",
	}

	k8sVersionsStrings := []string{
		"1.24.0",
		"1.24.1",
		"1.24.2",
		"1.24.3",
		"1.24.4",
		"1.24.5",
		"1.24.6",
		"1.24.7",
		"1.24.8",
		"1.24.9",
		"1.24.10",
		"1.24.11",
		"1.24.12",
		"1.24.13",
		"1.24.14",
		"1.24.15",
		"1.24.16",
		"1.25.0",
		"1.25.1",
		"1.25.2",
		"1.25.3",
		"1.25.4",
		"1.25.5",
		"1.25.6",
		"1.25.7",
		"1.25.8",
		"1.25.9",
		"1.25.10",
		"1.25.11",
		"1.25.12",
		"1.26.0",
		"1.26.1",
		"1.26.2",
		"1.26.3",
		"1.26.4",
		"1.26.5",
		"1.26.6",
		"1.26.7",
		"1.27.0",
		"1.27.1",
		"1.27.2",
		"1.27.3",
		"1.27.4",
	}

	k8sVersions := xslices.Map(k8sVersionsStrings, func(k8sVersion string) *compatibility.KubernetesVersion {
		version, err := compatibility.ParseKubernetesVersion(k8sVersion)
		if err != nil {
			panic(err)
		}

		return version
	})

	b.ReportAllocs()
	b.StartTimer()

	for b.Loop() {
		err := omni.ForAllCompatibleVersions(talosVersions, k8sVersions, func(string, []string) error {
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
