// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import "encoding/json"

// severityCounts is the number of findings in a report at each severity. Findings at a severity
// this doesn't track by name (e.g. Negligible, Unknown) are counted in Other.
type severityCounts struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Other    int
}

type grypeReport struct {
	Matches []struct {
		Vulnerability struct {
			Severity string `json:"severity"`
		} `json:"vulnerability"`
	} `json:"matches"`
}

// countSeverities parses a Grype vulnerability scan report (the JSON format the image factory
// serves) and counts its findings by severity, without needing a full mirror of its schema - only
// the one field this needs, matches[].vulnerability.severity, is decoded.
func countSeverities(data []byte) (severityCounts, error) {
	var r grypeReport

	if err := json.Unmarshal(data, &r); err != nil {
		return severityCounts{}, err
	}

	var counts severityCounts

	for _, m := range r.Matches {
		switch m.Vulnerability.Severity {
		case "Critical":
			counts.Critical++
		case "High":
			counts.High++
		case "Medium":
			counts.Medium++
		case "Low":
			counts.Low++
		default:
			counts.Other++
		}
	}

	return counts, nil
}
