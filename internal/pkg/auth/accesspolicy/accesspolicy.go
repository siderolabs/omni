// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package accesspolicy provides functionality to check and validate access policies.
package accesspolicy

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// GroupPrefix is the special prefix used in the AccessPolicy rules to denote a group of users or clusters.
const GroupPrefix = "group/"

// CheckResult is the result of an access policy check.
type CheckResult struct {
	Role                        role.Role
	KubernetesImpersonateGroups []string
	MatchesAllClusters          bool
}

// Validate validates the given access policy by running all its tests.
//
//nolint:gocognit,gocyclo,cyclop
func Validate(accessPolicy *auth.AccessPolicy) error {
	var validationErrs error

	// check metadata
	if accessPolicy.Metadata().ID() != auth.AccessPolicyID {
		validationErrs = multierror.Append(validationErrs, fmt.Errorf(
			"access policy ID mismatch: expected %q, got %q",
			auth.AccessPolicyID,
			accessPolicy.Metadata().ID(),
		))
	}

	if accessPolicy.Metadata().Namespace() != resources.DefaultNamespace {
		validationErrs = multierror.Append(validationErrs, fmt.Errorf(
			"access policy namespace mismatch: expected %q, got %q",
			resources.DefaultNamespace,
			accessPolicy.Metadata().Namespace(),
		))
	}

	accessPolicySpec := accessPolicy.TypedSpec().Value

	// check user groups
	for name, userGroup := range accessPolicySpec.UserGroups {
		for _, user := range userGroup.GetUsers() {
			numSetFields := 0

			if user.GetName() != "" {
				numSetFields++
			}

			if user.GetMatch() != "" {
				numSetFields++

				// check the validity of the match pattern
				if _, err := filepath.Match(user.GetMatch(), ""); err != nil {
					validationErrs = multierror.Append(validationErrs, err)
				}
			}

			if len(user.GetLabelSelectors()) != 0 {
				numSetFields++
			}

			if numSetFields == 0 {
				validationErrs = multierror.Append(validationErrs, fmt.Errorf(
					"access policy user group %q contains an empty user",
					name,
				))
			} else if numSetFields > 1 {
				validationErrs = multierror.Append(validationErrs, fmt.Errorf(
					"access policy user group %q contains a user with mutually exclusive fields set",
					name,
				))
			}
		}
	}

	// check cluster groups
	for name, clusterGroup := range accessPolicySpec.ClusterGroups {
		for _, cluster := range clusterGroup.GetClusters() {
			numSetFields := 0

			if cluster.GetName() != "" {
				numSetFields++
			}

			if cluster.GetMatch() != "" {
				numSetFields++

				// check the validity of the match pattern
				if _, err := filepath.Match(cluster.GetMatch(), ""); err != nil {
					validationErrs = multierror.Append(validationErrs, err)
				}
			}

			if numSetFields == 0 {
				validationErrs = multierror.Append(validationErrs, fmt.Errorf(
					"access policy cluster group %q contains an empty cluster",
					name,
				))
			} else if numSetFields > 1 {
				validationErrs = multierror.Append(validationErrs, fmt.Errorf(
					"access policy cluster group %q contains a cluster with mutually exclusive fields set",
					name,
				))
			}
		}
	}

	// check rules
	for _, rule := range accessPolicySpec.GetRules() {
		if rule.Role != "" {
			if _, err := role.Parse(rule.Role); err != nil {
				validationErrs = multierror.Append(validationErrs, err)
			}
		}
	}

	// check tests
	for _, test := range accessPolicySpec.GetTests() {
		testName := test.GetName()
		clusterName := test.GetCluster().GetName()
		userName := test.GetUser().GetName()

		// check test fields
		if testName == "" {
			validationErrs = multierror.Append(validationErrs, errors.New("invalid test: name is empty"))
		}

		if clusterName == "" {
			validationErrs = multierror.Append(validationErrs, fmt.Errorf("invalid test %q: cluster name is empty", clusterName))
		}

		if userName == "" {
			validationErrs = multierror.Append(validationErrs, fmt.Errorf("invalid test %q: user name is empty", userName))
		}

		// evaluate the test
		clusterMD := omni.NewCluster(clusterName).Metadata()
		identityMD := auth.NewIdentity(userName).Metadata()

		for key, value := range test.GetUser().GetLabels() {
			identityMD.Labels().Set(key, value)
		}

		checkResult, err := Check(accessPolicy, clusterMD, identityMD)
		if err != nil {
			validationErrs = multierror.Append(validationErrs, err)

			continue
		}

		expectedRole := test.GetExpected().GetRole()
		expectedImpersonateGroups := append([]string(nil), test.GetExpected().GetKubernetes().GetImpersonate().GetGroups()...)

		slices.Sort(expectedImpersonateGroups)
		slices.Sort(checkResult.KubernetesImpersonateGroups)

		if !slices.Equal(expectedImpersonateGroups, checkResult.KubernetesImpersonateGroups) {
			validationErrs = multierror.Append(validationErrs, fmt.Errorf(
				"access policy test %q failed: kubernetes impersonate groups mismatch: expected %q, got %q",
				test.GetName(),
				expectedImpersonateGroups,
				checkResult.KubernetesImpersonateGroups,
			))
		}

		if expectedRole != "" && string(checkResult.Role) != expectedRole {
			validationErrs = multierror.Append(validationErrs, fmt.Errorf(
				"access policy test %q failed: role mismatch: expected %q, got %q",
				test.GetName(),
				expectedRole,
				checkResult.Role,
			))
		}
	}

	return validationErrs
}

// Check checks the given user against the given cluster, and returns the result of the check, containing
// which role is assumed and which groups will be impersonated when the Kubernetes cluster is accessed.
//
//nolint:gocognit,gocyclo,cyclop
func Check(accessPolicy *auth.AccessPolicy, clusterMD, identityMD *resource.Metadata) (CheckResult, error) {
	if identityMD == nil {
		return CheckResult{}, errors.New("no user metadata")
	}

	if clusterMD == nil {
		return CheckResult{}, errors.New("no cluster metadata")
	}

	accessPolicySpec := accessPolicy.TypedSpec().Value
	maxRole := role.None

	if len(accessPolicySpec.GetRules()) == 0 {
		return CheckResult{
			Role: maxRole,
		}, nil
	}

	impersonateGroups := make([]string, 0, len(accessPolicySpec.GetRules()))

	match := func(md *resource.Metadata, exactMatchValue, matchPattern string, selectors []string) (bool, error) {
		if exactMatchValue != "" && md.ID() == exactMatchValue {
			return true, nil
		}

		if matchPattern != "" {
			matches, err := filepath.Match(matchPattern, md.ID())
			if err != nil {
				return false, fmt.Errorf("invalid match pattern %q for %s", matchPattern, md)
			}

			if matches {
				return true, nil
			}
		}

		if len(selectors) != 0 && md.Labels() != nil {
			query, err := labels.ParseSelectors([]string{strings.Join(selectors, ",")})
			if err != nil {
				return false, err
			}

			if query.Matches(*md.Labels()) {
				return true, nil
			}
		}

		return false, nil
	}

	matchesAllClusters := false

	for _, rule := range accessPolicySpec.GetRules() {
		userMatches := false

		for _, ruleUser := range rule.GetUsers() {
			if ruleUser == identityMD.ID() {
				userMatches = true

				break
			}

			if strings.HasPrefix(ruleUser, GroupPrefix) {
				groupName := ruleUser[len(GroupPrefix):]

				group, groupOk := accessPolicySpec.GetUserGroups()[groupName]
				if !groupOk {
					continue
				}

				for _, groupUser := range group.GetUsers() {
					matches, err := match(identityMD, groupUser.GetName(), groupUser.GetMatch(), groupUser.GetLabelSelectors())
					if err != nil {
						return CheckResult{}, err
					}

					if matches {
						userMatches = true

						break
					}
				}
			}
		}

		if !userMatches {
			continue
		}

		clusterMatches := false

		for _, ruleCluster := range rule.GetClusters() {
			if ruleCluster == clusterMD.ID() {
				clusterMatches = true

				break
			}

			if strings.HasPrefix(ruleCluster, GroupPrefix) {
				groupName := ruleCluster[len(GroupPrefix):]

				group, groupOk := accessPolicySpec.GetClusterGroups()[groupName]
				if !groupOk {
					continue
				}

				for _, groupCluster := range group.GetClusters() {
					if groupCluster.GetMatch() == "*" {
						clusterMatches = true
						matchesAllClusters = true

						break
					}

					matches, err := match(clusterMD, groupCluster.GetName(), groupCluster.GetMatch(), nil)
					if err != nil {
						return CheckResult{}, err
					}

					if matches {
						clusterMatches = true

						break
					}
				}
			}
		}

		if !clusterMatches {
			continue
		}

		if rule.Role != "" {
			parsedRole, err := role.Parse(rule.Role)
			if err != nil {
				return CheckResult{}, err
			}

			if parsedRole.Check(maxRole) == nil {
				maxRole = parsedRole
			}
		}

		impersonateGroups = append(impersonateGroups, rule.GetKubernetes().GetImpersonate().GetGroups()...)
	}

	return CheckResult{
		MatchesAllClusters:          matchesAllClusters,
		Role:                        maxRole,
		KubernetesImpersonateGroups: impersonateGroups,
	}, nil
}
