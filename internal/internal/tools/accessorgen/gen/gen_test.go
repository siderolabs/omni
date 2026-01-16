// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package gen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/omni/internal/internal/tools/accessorgen/gen"
)

func TestRun(t *testing.T) {
	// Setup
	inputContent := `package testdata

import "time"

type Simple struct {
	Name *string
	Age  *int
}

type Complex struct {
	Tags    *[]string
	Timeout *time.Duration
	// Should not generate for these
	Normal string
	Slice  []int
}
`
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.go")
	if err := os.WriteFile(inputFile, []byte(inputContent), 0o644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Run
	outputFile := filepath.Join(dir, "input.gen.go")
	if err := gen.Run(inputFile, outputFile); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	output := string(content)

	expectedChecks := []string{
		"func (s *Simple) GetName() string",
		"func (s *Simple) SetName(v string)",
		"func (s *Complex) GetTags() []string",
		"func (s *Complex) SetTags(v []string)",
		"func (s *Complex) GetTimeout() time.Duration",
		"func (s *Complex) SetTimeout(v time.Duration)",
	}

	for _, check := range expectedChecks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing expected code: %s", check)
		}
	}

	unexpectedChecks := []string{
		"GetNormal",
		"SetNormal",
		"GetSlice",
		"SetSlice",
	}

	for _, check := range unexpectedChecks {
		if strings.Contains(output, check) {
			t.Errorf("output contains unexpected code: %s", check)
		}
	}
}
