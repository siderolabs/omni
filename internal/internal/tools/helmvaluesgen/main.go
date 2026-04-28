// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.tools

package main

import (
	"flag"
	"log"

	"github.com/siderolabs/omni/internal/internal/tools/helmvaluesgen/gen"
)

func main() {
	schema := flag.String("schema", "", "JSON schema file to read")
	overrides := flag.String("overrides", "", "Helm config override file to read")
	values := flag.String("values", "", "Helm values.yaml file to update")

	flag.Parse()

	if err := gen.Run(*schema, *overrides, *values); err != nil {
		log.Fatal(err)
	}
}
