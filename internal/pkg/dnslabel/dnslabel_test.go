// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package dnslabel_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/dnslabel"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"a", "ab", "aws-1", "qemu-1", "abc123", "abc-123-def", strings.Repeat("a", dnslabel.MaxLength)} {
		t.Run("accept/"+name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, dnslabel.Validate(name))
			assert.True(t, dnslabel.IsValid(name))
		})
	}

	rejectCases := []struct {
		name    string
		input   string
		message string
	}{
		{"empty", "", "not a valid DNS-1123 label"},
		{"whitespace", "foo bar", "not a valid DNS-1123 label"},
		{"uppercase", "AWS-1", "not a valid DNS-1123 label"},
		{"underscore", "aws_1", "not a valid DNS-1123 label"},
		{"dot", "foo.bar", "not a valid DNS-1123 label"},
		{"colon", "infra-provider:foo", "not a valid DNS-1123 label"},
		{"leading hyphen", "-aws", "not a valid DNS-1123 label"},
		{"trailing hyphen", "aws-", "not a valid DNS-1123 label"},
		{"non-ascii", "qemü-1", "not a valid DNS-1123 label"},
		{"too long", strings.Repeat("a", dnslabel.MaxLength+1), "DNS-1123 label is too long"},
	}

	for _, tc := range rejectCases {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			t.Parallel()

			err := dnslabel.Validate(tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
			assert.False(t, dnslabel.IsValid(tc.input))
		})
	}

	t.Run("error for over-length input does not echo the value", func(t *testing.T) {
		t.Parallel()

		input := strings.Repeat("a", dnslabel.MaxLength*10)

		err := dnslabel.Validate(input)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), input)
	})
}
