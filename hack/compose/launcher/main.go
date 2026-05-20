// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"syscall"
)

// This is a simple launcher for development purposes.
// Execs the main Omni binary, optionally under Delve for debugging.
func main() {
	args := os.Args[1:]
	if os.Getenv("WITH_DEBUG") == "1" {
		dlvArgs := append([]string{
			"/debug/dlv", "exec",
			"--headless", "--listen=:12345",
			"--api-version=2", "--accept-multiclient",
			"--continue", "/omni", "--",
		}, args...)
		if err := syscall.Exec("/debug/dlv", dlvArgs, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "launcher: exec dlv:", err)
			os.Exit(1)
		}
	} else {
		if err := syscall.Exec("/omni", append([]string{"/omni"}, args...), os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "launcher: exec omni:", err)
			os.Exit(1)
		}
	}
}
