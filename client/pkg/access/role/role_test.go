// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package role_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/access/role"
)

var all = []role.Role{role.None, role.InfraProvider, role.Reader, role.Auditor, role.Operator, role.Admin}

func TestParse(t *testing.T) {
	for _, r := range all {
		parsed, err := role.Parse(string(r))
		require.NoError(t, err)
		assert.Equal(t, r, parsed)
	}

	_, err := role.Parse("Nonexistent")
	assert.Error(t, err)

	_, err = role.Parse("")
	assert.Error(t, err)
}

// TestCheckMatrix pins the full ordering, so that inserting or reordering a role cannot silently change
// who satisfies what.
func TestCheckMatrix(t *testing.T) {
	// expected[actor][required] reports whether actor satisfies required.
	expected := map[role.Role]map[role.Role]bool{
		role.None:          {role.None: true, role.InfraProvider: false, role.Reader: false, role.Auditor: false, role.Operator: false, role.Admin: false},
		role.InfraProvider: {role.None: true, role.InfraProvider: true, role.Reader: false, role.Auditor: false, role.Operator: false, role.Admin: false},
		role.Reader:        {role.None: true, role.InfraProvider: true, role.Reader: true, role.Auditor: false, role.Operator: false, role.Admin: false},
		role.Auditor:       {role.None: true, role.InfraProvider: true, role.Reader: true, role.Auditor: true, role.Operator: false, role.Admin: false},
		role.Operator:      {role.None: true, role.InfraProvider: true, role.Reader: true, role.Auditor: true, role.Operator: true, role.Admin: false},
		role.Admin:         {role.None: true, role.InfraProvider: true, role.Reader: true, role.Auditor: true, role.Operator: true, role.Admin: true},
	}

	for _, actor := range all {
		for _, required := range all {
			assert.Equal(t, expected[actor][required], actor.Check(required) == nil,
				"actor %q checking %q", actor, required)
		}
	}
}

// TestAuditorIsNotWriter is the regression that keeps an Auditor away from write access, and with it away
// from the OIDC system:masters group, which is granted on a successful Check(Operator).
func TestAuditorIsNotWriter(t *testing.T) {
	assert.NoError(t, role.Auditor.Check(role.Reader), "Auditor must retain read access")
	assert.Error(t, role.Auditor.Check(role.Operator), "Auditor must not satisfy Operator")
	assert.Error(t, role.Auditor.Check(role.Admin), "Auditor must not satisfy Admin")
}

// TestOperatorOutranksAuditor documents why audit log access cannot be a threshold check. Operator sits
// above Auditor in the ordering, so a Check-based gate would hand the audit log to every Operator. The
// audit log is therefore gated by exact role at its call sites.
func TestOperatorOutranksAuditor(t *testing.T) {
	assert.NoError(t, role.Operator.Check(role.Auditor))
	assert.NoError(t, role.Admin.Check(role.Auditor))
}

func TestCanReadAuditLog(t *testing.T) {
	for _, r := range all {
		want := r == role.Auditor || r == role.Admin
		assert.Equal(t, want, role.CanReadAuditLog(r), "CanReadAuditLog(%q)", r)
	}

	assert.False(t, role.CanReadAuditLog(role.Operator),
		"Operator outranks Auditor but must never read the audit log")
}

// TestMinDoesNotSynthesizeAuditAccess is the regression for a privilege inversion. Auditor sits below
// Operator, so capping by rank alone made Min(Operator, Auditor) return Auditor. An Operator could then
// register a public key as Auditor, or keep an old Auditor key after being changed to Operator, and read
// the audit log with an effective role neither input legitimately held.
func TestMinDoesNotSynthesizeAuditAccess(t *testing.T) {
	for _, other := range []role.Role{role.None, role.InfraProvider, role.Reader, role.Operator} {
		got, err := role.Min(role.Auditor, other)
		require.NoError(t, err)
		assert.False(t, role.CanReadAuditLog(got),
			"Min(Auditor, %q) = %q must not be able to read the audit log", other, got)

		got, err = role.Min(other, role.Auditor)
		require.NoError(t, err)
		assert.False(t, role.CanReadAuditLog(got),
			"Min(%q, Auditor) = %q must not be able to read the audit log", other, got)
	}

	// audit access survives only when every input has it.
	for _, pair := range [][2]role.Role{{role.Auditor, role.Auditor}, {role.Auditor, role.Admin}, {role.Admin, role.Auditor}} {
		got, err := role.Min(pair[0], pair[1])
		require.NoError(t, err)
		assert.Equal(t, role.Auditor, got, "Min(%q, %q)", pair[0], pair[1])
	}
}

// TestPrevious pins the ordering as seen by Previous, which the integration auth matrix uses to derive
// its "insufficient role" cases. Inserting Auditor moves Operator's predecessor from Reader to Auditor,
// and both are equally valid negative cases because neither satisfies Operator.
func TestPrevious(t *testing.T) {
	for _, tt := range []struct {
		in, want role.Role
	}{
		{role.InfraProvider, role.None},
		{role.Reader, role.InfraProvider},
		{role.Auditor, role.Reader},
		{role.Operator, role.Auditor},
		{role.Admin, role.Operator},
	} {
		got, err := tt.in.Previous()
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "Previous(%q)", tt.in)
	}

	_, err := role.None.Previous()
	assert.Error(t, err, "the lowest role has no predecessor")
}

func TestMinMax(t *testing.T) {
	for _, tt := range []struct {
		a, b, min, max role.Role
	}{
		{role.Reader, role.Admin, role.Reader, role.Admin},
		{role.Auditor, role.Admin, role.Auditor, role.Admin},
		{role.Auditor, role.Reader, role.Reader, role.Auditor},
		// not Auditor: mixing with a role that cannot read the audit log drops that access.
		{role.Auditor, role.Operator, role.Reader, role.Operator},
		{role.None, role.Auditor, role.None, role.Auditor},
	} {
		minRole, err := role.Min(tt.a, tt.b)
		require.NoError(t, err)
		assert.Equal(t, tt.min, minRole, "Min(%q, %q)", tt.a, tt.b)

		maxRole, err := role.Max(tt.a, tt.b)
		require.NoError(t, err)
		assert.Equal(t, tt.max, maxRole, "Max(%q, %q)", tt.a, tt.b)
	}
}
