// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"fmt"
	"strings"
	"unicode"
)

// validateUserString caps the byte length of a user-supplied string and rejects control
// characters. Empty values are accepted.
func validateUserString(fieldName, value string, maxLen int) error {
	if value == "" {
		return nil
	}

	if len(value) > maxLen {
		return fmt.Errorf("%s is too long: %d bytes (max %d)", fieldName, len(value), maxLen)
	}

	if strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("%s contains a control character", fieldName)
	}

	return nil
}

// validateUserStringSlice caps the number of entries in a list of user-supplied strings and
// runs validateUserString on each entry.
func validateUserStringSlice(fieldName string, values []string, maxCount, maxElemLen int) error {
	if len(values) > maxCount {
		return fmt.Errorf("%s has too many entries: %d (max %d)", fieldName, len(values), maxCount)
	}

	for i, v := range values {
		if err := validateUserString(fmt.Sprintf("%s[%d]", fieldName, i), v, maxElemLen); err != nil {
			return err
		}
	}

	return nil
}
