// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import dto "github.com/prometheus/client_model/go"

type MyStruct struct {
	Val  string
	Data string
}

// Equal returns true if the value is equal to the given value.
func (s *MyStruct) Equal(v *MyStruct) bool {
	return s.Val == v.Val
}

type ActionMap = actionMap[string, string]

type UniqueSlice = uniqueSlice[*MyStruct]

func (rec *Reconciler) IdleConnections() float64 {
	m := &dto.Metric{}

	if err := rec.totalConnections.Write(m); err != nil {
		panic(err)
	}

	return m.GetGauge().GetValue()
}
