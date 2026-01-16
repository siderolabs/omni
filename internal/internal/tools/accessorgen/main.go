// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package main

import (
	"flag"
	"log"

	"github.com/siderolabs/omni/internal/internal/tools/accessorgen/gen"
)

func main() {
	source := flag.String("source", "", "Go source file to process")
	output := flag.String("output", "", "Output generated file")

	flag.Parse()

	if err := gen.Run(*source, *output); err != nil {
		log.Fatal(err)
	}
}
