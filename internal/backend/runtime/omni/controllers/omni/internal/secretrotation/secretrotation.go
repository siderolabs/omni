// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package secretrotation includes helpers for secret rotation.
package secretrotation

import (
	"slices"
)

// Candidates is a list of candidates for rotation.
type Candidates struct {
	Candidates []Candidate
}

// Add adds a candidate to the list.
func (c *Candidates) Add(candidate Candidate) {
	c.Candidates = append(c.Candidates, candidate)
}

// Len returns the number of candidates.
func (c *Candidates) Len() int {
	return len(c.Candidates)
}

// Sort sorts the candidates by control plane, ready, and hostname.
func (c *Candidates) Sort() {
	slices.SortFunc(c.Candidates, func(a, b Candidate) int {
		switch {
		case a.Less(b):
			return -1
		case b.Less(a):
			return +1
		default:
			return 0
		}
	})
}

type Filter int

const (
	Parallel Filter = iota
	Serial
)

// Viable returns two slices of candidates after applying the given filters:
// This function takes Machine's type, readiness, and lock status into account.
//   - Candidates that are viable for secret rotation
//   - Candidates that require secret rotation but blocked until some requirements are met
func (c *Candidates) Viable(controlPlaneFilter Filter, workerFilter Filter) (viable []Candidate, blocked []Candidate) {
	var (
		viableCP  []Candidate
		viableW   []Candidate
		blockedCP []Candidate
		blockedW  []Candidate
	)

	c.Sort()

	for _, candidate := range c.Candidates {
		if !candidate.Ready || candidate.Locked {
			if candidate.ControlPlane {
				blockedCP = append(blockedCP, candidate)
			} else {
				blockedW = append(blockedW, candidate)
			}

			continue
		}

		if candidate.ControlPlane {
			viableCP = append(viableCP, candidate)

			continue
		}

		viableW = append(viableW, candidate)
	}

	switch controlPlaneFilter {
	case Parallel:
		viable = append(viable, viableCP...)
		blocked = append(blocked, blockedCP...)
	case Serial:
		for i, candidate := range viableCP {
			if i == 0 {
				viable = append(viable, candidate)

				continue
			}

			blocked = append(blocked, candidate)
		}

		blocked = append(blocked, blockedCP...)
	}

	if len(viable) > 0 || len(blocked) > 0 {
		return viable, blocked
	}

	switch workerFilter {
	case Parallel:
		viable = append(viable, viableW...)
		blocked = append(blocked, blockedW...)
	case Serial:
		for i, candidate := range viableW {
			if i == 0 {
				viable = append(viable, candidate)

				continue
			}

			blocked = append(blocked, candidate)
		}

		blocked = append(blocked, blockedW...)
	}

	return viable, blocked
}

// Locked returns the list of blocked machines. It returns a slice of control planes if there are locked control planes, otherwise it returns a slice of locked workers.
// This method call is informational. We first want to inform the caller about control planes, because having the knowledge of locked workers doesn't bring any benefit.
func (c *Candidates) Locked() []string {
	return c.filter(func(candidate Candidate) bool {
		return candidate.Locked
	})
}

// NotReady returns the list of machines that are not ready. It returns a slice of control planes if there are non-ready control planes, otherwise it returns a slice of non-ready workers.
// This method call is informational. We first want to inform the caller about control planes, because having the knowledge of unhealthy workers doesn't bring any benefit.
func (c *Candidates) NotReady() []string {
	return c.filter(func(candidate Candidate) bool {
		return !candidate.Ready
	})
}

func (c *Candidates) filter(filterFunc func(candidate Candidate) bool) []string {
	var cp, w []string

	for _, candidate := range c.Candidates {
		if filterFunc(candidate) {
			if candidate.ControlPlane {
				cp = append(cp, candidate.Hostname)
			} else {
				w = append(w, candidate.Hostname)
			}
		}
	}

	if len(cp) > 0 {
		return cp
	}

	return w
}

// Candidate is a candidate for rotation.
type Candidate struct {
	RemoteGeneratorFactory  RemoteGeneratorFactory
	KubernetesClientFactory KubernetesClientFactory
	MachineID               string
	Hostname                string
	ControlPlane            bool
	Locked                  bool
	Ready                   bool
}

// Less returns true if the candidate should be rotated before the other one.
func (c Candidate) Less(other Candidate) bool {
	if c.ControlPlane != other.ControlPlane {
		return c.ControlPlane
	}

	if c.Ready != other.Ready {
		return c.Ready
	}

	if c.Locked != other.Locked {
		return !c.Locked
	}

	return c.Hostname < other.Hostname
}
