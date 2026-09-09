// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omnictl/security"
)

func TestOutputFlagValidate(t *testing.T) {
	require.NoError(t, security.NewOutputFlag("json").Validate("json", "yaml"))
	require.NoError(t, security.NewOutputFlag("yaml").Validate("json", "yaml"))
	require.ErrorContains(t, security.NewOutputFlag("table").Validate("json", "yaml"), `invalid --output "table": must be one of json, yaml`)

	require.NoError(t, security.NewOutputFlag("table").Validate("table", "json", "yaml"))
	require.ErrorContains(t, security.NewOutputFlag("bogus").Validate("table", "json", "yaml"), `invalid --output "bogus": must be one of table, json, yaml`)
}

func TestRawJSON(t *testing.T) {
	data, err := security.RawJSON([]byte(`{"a": 1, "b": [true, "c"]}`))
	require.NoError(t, err)
	// Embedded verbatim, byte-for-byte - not decoded and re-encoded.
	require.Equal(t, json.RawMessage(`{"a": 1, "b": [true, "c"]}`), data)

	_, err = security.RawJSON([]byte("not json"))
	require.ErrorContains(t, err, "invalid JSON")
}

// TestWriteJSONPreservesFactoryOutput covers the mechanism -o json's byte-exact passthrough relies
// on: the encoder only adjusts whitespace around bytes embedded via json.RawMessage, so a report's
// key order and numbers survive unchanged - unlike decoding through a generic map[string]any (whose
// keys json.Marshal always sorts) and re-encoding (whose numbers all become float64, losing
// precision past 2^53).
func TestWriteJSONPreservesFactoryOutput(t *testing.T) {
	report, err := security.RawJSON([]byte(`{"z_field":1,"a_field":9007199254740993,"nested":{"b":2,"a":1}}`))
	require.NoError(t, err)

	var buf bytes.Buffer

	require.NoError(t, security.WriteJSON(&buf, security.ScanResult{Version: "1.9.0", Report: report}))

	require.Contains(t, buf.String(), `"z_field": 1,`)
	require.Contains(t, buf.String(), `"a_field": 9007199254740993,`)
	require.Contains(t, buf.String(), `"nested": {`)
}

// TestWriteJSONDoesNotEscapeHTML covers the other half of -o json's passthrough guarantee:
// json.Marshal would rewrite &, < and > inside the embedded factory bytes as \u escapes, and all
// three are routine in SBOM reference URLs and scan descriptions, so essentially every real report
// would come out modified.
func TestWriteJSONDoesNotEscapeHTML(t *testing.T) {
	report, err := security.RawJSON([]byte(`{"url":"https://x/y?a=1&b=2","d":">= 2.34, <= 2.43"}`))
	require.NoError(t, err)

	var buf bytes.Buffer

	require.NoError(t, security.WriteJSON(&buf, security.ScanResult{Version: "1.9.0", Report: report}))

	require.Contains(t, buf.String(), `"url": "https://x/y?a=1&b=2"`)
	require.Contains(t, buf.String(), `"d": ">= 2.34, <= 2.43"`)
	require.NotContains(t, buf.String(), `\u0026`)
}

// TestWriteJSONKeepsUnprintableBytesValid covers what sending structured output through safeout
// used to break: safeout escapes every rune unicode.IsPrint rejects - which includes invalid UTF-8
// and any code point unassigned in Go's Unicode tables - and rewriting those bytes inside an
// already-encoded document leaves output that no longer parses as JSON at all. An SBOM package
// description is an arbitrary string from an upstream project, so this is not a hypothetical.
func TestWriteJSONKeepsUnprintableBytesValid(t *testing.T) {
	report, err := security.RawJSON([]byte("{\"d\":\"tag\U000e0001end\",\"e\":\"bad\xffbyte\"}"))
	require.NoError(t, err)

	var buf bytes.Buffer

	require.NoError(t, security.WriteJSON(&buf, security.ScanResult{Version: "1.9.0", Report: report}))

	require.True(t, json.Valid(buf.Bytes()), "output must stay parseable: %q", buf.Bytes())
}

func TestParseReportFormat(t *testing.T) {
	format, err := security.ParseReportFormat("json")
	require.NoError(t, err)
	require.Equal(t, imagefactorypb.VulnerabilityReportFormat_JSON, format)

	format, err = security.ParseReportFormat("sarif")
	require.NoError(t, err)
	require.Equal(t, imagefactorypb.VulnerabilityReportFormat_SARIF, format)

	format, err = security.ParseReportFormat("cyclonedx")
	require.NoError(t, err)
	require.Equal(t, imagefactorypb.VulnerabilityReportFormat_CYCLONEDX, format)

	_, err = security.ParseReportFormat("table")
	require.ErrorContains(t, err, `invalid --format "table"`)
}

func TestValidateScanOutputFormat(t *testing.T) {
	require.NoError(t, security.ValidateScanOutputFormat("table", imagefactorypb.VulnerabilityReportFormat_JSON))
	require.NoError(t, security.ValidateScanOutputFormat("json", imagefactorypb.VulnerabilityReportFormat_SARIF))
	require.NoError(t, security.ValidateScanOutputFormat("yaml", imagefactorypb.VulnerabilityReportFormat_CYCLONEDX))

	err := security.ValidateScanOutputFormat("table", imagefactorypb.VulnerabilityReportFormat_SARIF)
	require.ErrorContains(t, err, "-o table only works with --format json")

	err = security.ValidateScanOutputFormat("table", imagefactorypb.VulnerabilityReportFormat_CYCLONEDX)
	require.ErrorContains(t, err, "-o table only works with --format json")
}

func TestShortSchematic(t *testing.T) {
	require.Equal(t, "abc", security.ShortSchematic("abc"))
	require.Equal(t, "376567988ad3…", security.ShortSchematic("376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"))
}
