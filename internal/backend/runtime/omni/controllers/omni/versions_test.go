// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
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

// stubFactoryClient is a minimal imagefactory.FactoryClient used to drive fetchTalosVersions. It is
// defined here (rather than reusing testutils) because testutils imports this package.
type stubFactoryClient struct {
	err      error
	url      string
	versions []string
}

func (s *stubFactoryClient) URL() string { return s.url }

func (s *stubFactoryClient) Host() string { return s.url }

func (s *stubFactoryClient) Versions(context.Context) ([]string, error) {
	return s.versions, s.err
}

func (s *stubFactoryClient) CachedIsEnterprise() bool { return false }

func (s *stubFactoryClient) EnsureSchematic(context.Context, schematic.Schematic) (string, *schematic.Schematic, error) {
	return "", nil, nil
}

func (s *stubFactoryClient) SchematicGet(context.Context, string) (*schematic.Schematic, error) {
	return nil, nil //nolint:nilnil
}

func (s *stubFactoryClient) OverlaysVersions(context.Context, string) ([]client.OverlayInfo, error) {
	return nil, nil
}

func (s *stubFactoryClient) ExtensionsVersions(context.Context, string) ([]client.ExtensionInfo, error) {
	return nil, nil
}

func (s *stubFactoryClient) TalosctlList(context.Context, string) ([]string, error) { return nil, nil }

func (s *stubFactoryClient) ScanReport(context.Context, string, string, string, string) ([]byte, error) {
	return nil, nil
}

func (s *stubFactoryClient) SPDXBundle(context.Context, string, string, string) ([]byte, error) {
	return nil, nil
}

func (s *stubFactoryClient) VEXDocument(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *stubFactoryClient) DownloadToken(context.Context, time.Duration) (string, error) {
	return "", &client.HTTPError{Code: http.StatusNotFound, Message: "not found"}
}

// TestFetchTalosVersions verifies the two configured factories are independent: a failure fetching one
// never fails the other, the failed factory's URL is reported so its versions can be preserved, and the
// primary still overwrites the secondary for versions present in both.
func TestFetchTalosVersions(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	const (
		primaryURL   = "https://primary.test"
		secondaryURL = "https://secondary.test"
	)

	newClients := func(primary, secondary *stubFactoryClient) *imagefactory.Clients {
		clients := imagefactory.NewClients(nil, primary)
		if secondary != nil {
			clients.SetSecondary(secondary)
		}

		return clients
	}

	t.Run("both succeed: primary overwrites shared versions, nothing failed", func(t *testing.T) {
		t.Parallel()

		clients := newClients(
			&stubFactoryClient{url: primaryURL, versions: []string{"v1.13.0", "v1.13.5"}},
			&stubFactoryClient{url: secondaryURL, versions: []string{"v1.13.5", "v1.14.0"}},
		)

		versionToFactory, failed := omni.FetchTalosVersions(t.Context(), clients, logger)

		assert.Empty(t, failed)
		assert.Equal(t, primaryURL, versionToFactory["1.13.0"].FactoryURL)
		assert.Equal(t, primaryURL, versionToFactory["1.13.5"].FactoryURL) // primary overwrites the shared version
		assert.Equal(t, secondaryURL, versionToFactory["1.14.0"].FactoryURL)
	})

	t.Run("secondary fails: it is reported and the primary is unaffected", func(t *testing.T) {
		t.Parallel()

		clients := newClients(
			&stubFactoryClient{url: primaryURL, versions: []string{"v1.13.0"}},
			&stubFactoryClient{url: secondaryURL, err: errors.New("503 service unavailable")},
		)

		versionToFactory, failed := omni.FetchTalosVersions(t.Context(), clients, logger)

		assert.Contains(t, failed, secondaryURL)
		assert.NotContains(t, failed, primaryURL)
		assert.Equal(t, primaryURL, versionToFactory["1.13.0"].FactoryURL)
		assert.NotContains(t, versionToFactory, "1.14.0") // a secondary-only version is simply not observed this cycle
	})

	t.Run("primary fails: it is reported and the secondary is unaffected", func(t *testing.T) {
		t.Parallel()

		clients := newClients(
			&stubFactoryClient{url: primaryURL, err: errors.New("503 service unavailable")},
			&stubFactoryClient{url: secondaryURL, versions: []string{"v1.13.5", "v1.14.0"}},
		)

		versionToFactory, failed := omni.FetchTalosVersions(t.Context(), clients, logger)

		assert.Contains(t, failed, primaryURL)
		assert.NotContains(t, failed, secondaryURL)
		assert.Equal(t, secondaryURL, versionToFactory["1.13.5"].FactoryURL) // no primary result to overwrite it
		assert.Equal(t, secondaryURL, versionToFactory["1.14.0"].FactoryURL)
	})
}
