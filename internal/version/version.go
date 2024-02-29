// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
//
// Generated on 2024-02-23T01:26:28Z by kres 38f172c.

// Package version contains variables such as project name, tag and sha. It's a proper alternative to using
// -ldflags '-X ...'.
package version

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

var (
	// Tag declares project git tag.
	//go:embed data/tag
	Tag string
	// SHA declares project git SHA.
	//go:embed data/sha
	SHA string
	// Name declares project name.
	Name = func() string {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			panic("cannot read build info, something is very wrong")
		}

		// Check if siderolabs project
		if strings.HasPrefix(info.Path, "github.com/siderolabs/") {
			return info.Path[strings.LastIndex(info.Path, "/")+1:]
		}

		// We could return a proper full path here, but it could be seen as a privacy violation.
		return "community-project"
	}()
)
