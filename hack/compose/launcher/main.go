// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

const (
	WITH_DEBUG_ENV = "WITH_DEBUG"
	OMNI_BIN_ENV   = "OMNI_BIN"
)

// This is a simple launcher for development purposes.
// Execs the main Omni binary, optionally under Delve for debugging.
//
// Set WITH_DEBUG=true to enable the Delve debug server, defaults to false.
// Set OMNI_BIN to the path of the Omni binary to execute (required).
func main() {
	args := os.Args[1:]
	isDebug := false

	omniPath := os.Getenv(OMNI_BIN_ENV)
	if omniPath == "" {
		fmt.Fprintf(os.Stderr, "launcher: %s is required\n", OMNI_BIN_ENV)
		os.Exit(1)
	}

	if rawWithDebug := os.Getenv(WITH_DEBUG_ENV); rawWithDebug != "" {
		var err error
		if isDebug, err = strconv.ParseBool(rawWithDebug); err != nil {
			fmt.Fprintf(os.Stderr, "launcher: invalid %s=%q: %v\n", WITH_DEBUG_ENV, rawWithDebug, err)
			os.Exit(1)
		}
	}

	if isDebug {
		listenAddr := os.Getenv("DELVE_LISTEN_ADDR")
		if listenAddr == "" {
			listenAddr = "127.0.0.1:12345"
		}

		dlvArgs := append([]string{
			"/debug/dlv", "exec",
			"--headless", "--listen=" + listenAddr,
			"--api-version=2", "--accept-multiclient",
			"--continue", omniPath, "--",
		}, args...)
		if err := syscall.Exec("/debug/dlv", dlvArgs, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "launcher: exec dlv:", err)
			os.Exit(1)
		}
	} else {
		if err := syscall.Exec(omniPath, append([]string{omniPath}, args...), os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "launcher: exec omni:", err)
			os.Exit(1)
		}
	}
}
