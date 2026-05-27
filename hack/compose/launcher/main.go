// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package main

import (
	"log/slog"
	"os"
	"strconv"
	"syscall"
)

const (
	withDebugEnv       = "WITH_DEBUG"
	omniBinEnvName     = "OMNI_BIN"
	delveListenAddrEnv = "DELVE_LISTEN_ADDR"
)

// This is a simple launcher for development purposes.
// Execs the main Omni binary, optionally under Delve for debugging.
//
// Set WITH_DEBUG=true to enable the Delve debug server, defaults to false.
// Set OMNI_BIN to the path of the Omni binary to execute (required).
func main() {
	args := os.Args[1:]
	isDebug := false

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	omniPath := os.Getenv(omniBinEnvName)
	if omniPath == "" {
		logger.Error("env var is required", "name", omniBinEnvName)
		os.Exit(1)
	}

	if rawWithDebug := os.Getenv(withDebugEnv); rawWithDebug != "" {
		var err error
		if isDebug, err = strconv.ParseBool(rawWithDebug); err != nil {
			logger.Error("invalid env var value", "name", withDebugEnv, "value", rawWithDebug, "err", err)
			os.Exit(1)
		}
	}

	if isDebug {
		listenAddr := os.Getenv(delveListenAddrEnv)
		if listenAddr == "" {
			logger.Error("env var is required when debugging is enabled", "name", delveListenAddrEnv)
			os.Exit(1)
		}

		dlvArgs := append([]string{
			"/debug/dlv",
			"exec",
			omniPath,
			"--listen=" + listenAddr,
			"--headless",
			"--accept-multiclient",
			"--continue",
			"--",
		}, args...)
		logger.Info("launching dlv", "args", dlvArgs)

		if err := syscall.Exec("/debug/dlv", dlvArgs, os.Environ()); err != nil {
			logger.Error("exec dlv failed", "err", err)
			os.Exit(1)
		}
	} else {
		if err := syscall.Exec(omniPath, append([]string{omniPath}, args...), os.Environ()); err != nil {
			logger.Error("exec omni failed", "err", err)
			os.Exit(1)
		}
	}
}
