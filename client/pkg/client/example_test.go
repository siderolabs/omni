// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"context"
	"log"

	"github.com/cosi-project/runtime/pkg/safe"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/version"
)

//nolint:wsl,testableexamples
func Example() {
	// This example shows how to use Omni client to access resources.

	// Setup versions information. You can embed that into `go build` too.
	version.Name = "omni"
	version.SHA = "build SHA"
	version.Tag = "v0.9.1"

	// For this example we will use Omni service account.
	// You can create your service account in advance:
	//
	// omnictl serviceaccount create example.account
	// Created service account "example.account" with public key ID "<REDACTED>"
	//
	// Set the following environment variables to use the service account:
	// OMNI_ENDPOINT=https://<account>.omni.siderolabs.io:443
	// OMNI_SERVICE_ACCOUNT_KEY=base64encodedkey
	//
	// Note: Store the service account key securely, it will not be displayed again

	ctx := context.Background()

	// Creating a new client.
	client, err := client.New("https://<account>.omni.siderolabs.io:443", client.WithServiceAccount(
		"base64encodedkey", // From the generated service account.
	))
	if err != nil {
		log.Panicf("failed to create omni client %s", err)
	}

	// Omni service is using COSI https://github.com/cosi-project/runtime/.
	// The same client is used to get resources in Talos.
	st := client.Omni().State()

	defer func() {
		if e := client.Close(); e != nil {
			log.Printf("failed to close client %s", e)
		}
	}()

	// Getting the resources from the Omni state.
	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		log.Panicf("failed to get machines %s", err)
	}

	var (
		cluster string
		machine *omni.MachineStatus
	)

	for item := range machines.All() {
		log.Printf("machine %s, connected: %t", item.Metadata(), item.TypedSpec().Value.GetConnected())

		// Check cluster assignment for a machine.
		// Find a machine which is allocated into a cluster for the later use.
		if c, ok := item.Metadata().Labels().Get(omni.LabelCluster); ok && machine == nil {
			cluster = c
			machine = item
		}
	}

	// Creating an empty cluster via template.
	// Alternative is to use template.Load to load a cluster template.
	template := template.WithCluster("example.cluster")

	if _, err = template.Sync(ctx, st); err != nil {
		log.Panicf("failed to sync cluster %s", err)
	}
	if _, err = template.Sync(ctx, st); err != nil {
		log.Panicf("failed to sync cluster %s", err)
	}

	log.Printf("sync cluster")

	// Delete cluster.
	if _, err = template.Delete(ctx, st); err != nil {
		log.Panicf("failed to delete the cluster %s", err)
	}
	if _, err = template.Delete(ctx, st); err != nil {
		log.Panicf("failed to delete the cluster %s", err)
	}

	log.Printf("destroyed cluster")

	// No machines found, exit.
	if machine == nil {
		log.Printf("no allocated machines found, exit")

		return
	}

	// Using Talos through Omni.
	// Use cluster and machine which we previously found.
	cpuInfo, err := client.Talos().WithCluster(
		cluster,
	).WithNodes(
		machine.Metadata().ID(), // You can use machine UUID as Omni will properly resolve it into machine IP.
	).CPUInfo(ctx, &emptypb.Empty{})
	if err != nil {
		log.Panicf("failed to read machine CPU info %s", err)
	}

	for _, message := range cpuInfo.Messages {
		for i, info := range message.CpuInfo {
			log.Printf("machine %s, CPU %d family %s", machine.Metadata(), i, info.CpuFamily)
		}

		if len(message.CpuInfo) == 0 {
			log.Printf("no CPU info for machine %s", machine.Metadata())
		}
	}

	// Talking to Omni specific APIs: getting talosconfig.
	_, err = client.Management().Talosconfig(ctx)
	if err != nil {
		log.Panicf("failed to get talosconfig %s", err)
	}
}
