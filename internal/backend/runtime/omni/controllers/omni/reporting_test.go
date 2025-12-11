// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/suite"
	"github.com/stripe/stripe-go/v76"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type StripeMetricsReporterControllerSuite struct {
	OmniSuite
}

func (suite *StripeMetricsReporterControllerSuite) TestReconcile() {
	suite.startRuntime()

	// Register controller with a min commit of 4 machines
	suite.Require().NoError(
		suite.runtime.RegisterController(
			omnictrl.NewStripeMetricsReporterController("test_api_key", "sub_item_id", 4, omnictrl.WithDebounceDuration(2*time.Second)),
		),
	)

	var (
		machineCount int64 = 0
		mu           sync.Mutex
	)

	// Mock HTTP server to simulate Stripe API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)

		switch r.Method {
		case http.MethodGet:
			suite.Assert().Equal("/v1/subscription_items/sub_item_id", r.URL.Path)

			mu.Lock()

			count := machineCount

			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(map[string]any{
				"id":       "sub_item_id",
				"quantity": count,
			})
			suite.Require().NoError(err)

		case http.MethodPost:
			suite.Assert().Equal("/v1/subscription_items/sub_item_id", r.URL.Path)

			if err := r.ParseForm(); err != nil {
				log.Printf("Failed to parse form: %v", err)
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			log.Printf("Received form data: %v", r.Form)

			if quantityStr := r.Form.Get("quantity"); quantityStr != "" {
				if newCount, err := strconv.ParseInt(quantityStr, 10, 64); err == nil {
					mu.Lock()

					machineCount = newCount

					mu.Unlock()

					w.WriteHeader(http.StatusOK)
					err = json.NewEncoder(w).Encode(map[string]any{
						"id":       "sub_item_id",
						"quantity": machineCount,
					})
					suite.Require().NoError(err)
				} else {
					log.Printf("Invalid quantity value: %v", quantityStr)
					w.WriteHeader(http.StatusBadRequest)
				}
			} else {
				log.Printf("Missing quantity parameter in form data")
				w.WriteHeader(http.StatusBadRequest)
			}

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	// Use Stripe's MockBackend
	mockBackend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL: &mockServer.URL,
	})
	stripe.SetBackend(stripe.APIBackend, mockBackend)

	// Register 1 machine and wait for the debounce so we can ensure that the min commit is what's actually reported
	metrics := omni.NewMachineStatusMetrics(resources.EphemeralNamespace, omni.MachineStatusMetricsID)
	metrics.TypedSpec().Value.RegisteredMachinesCount = 1
	suite.Require().NoError(suite.state.Create(suite.ctx, metrics))

	// Allow time for the controller to reconcile
	time.Sleep(3 * time.Second)

	var result map[string]any

	// Verify the mock server returns the minimum machine count instead of the 1 we've registered.
	req, err := http.NewRequestWithContext(suite.ctx, http.MethodGet, mockServer.URL+"/v1/subscription_items/sub_item_id", nil)
	suite.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	suite.Require().NoError(err)

	//nolint: errcheck
	defer resp.Body.Close()

	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))

	//nolint: errcheck,forcetypeassert
	suite.Assert().Equal(int64(4), int64(result["quantity"].(float64)))

	// Update to count > min and ensure that is reflected
	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, metrics.Metadata(), func(r *omni.MachineStatusMetrics) error {
		r.TypedSpec().Value.RegisteredMachinesCount = 6

		return nil
	})
	suite.Require().NoError(err)

	// Allow time for the controller to reconcile
	time.Sleep(3 * time.Second)

	// Verify the mock server reflects the updated machine count
	req, err = http.NewRequestWithContext(suite.ctx, http.MethodGet, mockServer.URL+"/v1/subscription_items/sub_item_id", nil)
	suite.Require().NoError(err)

	resp, err = http.DefaultClient.Do(req)
	suite.Require().NoError(err)

	//nolint: errcheck
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	suite.Require().NoError(err)

	//nolint: errcheck,forcetypeassert
	suite.Assert().Equal(int64(6), int64(result["quantity"].(float64)))
}

func TestStripeMetricsReporterControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(StripeMetricsReporterControllerSuite))
}
