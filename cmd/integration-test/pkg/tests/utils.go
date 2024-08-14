// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"errors"
	"io"
	"reflect"
	"time"
)

type corpusEntry = struct {
	Parent     string
	Path       string
	Data       []byte
	Values     []any
	Generation int
	IsSeed     bool
}

var errMain = errors.New("testing: unexpected use of func Main")

type matchStringOnly func(pat, str string) (bool, error)

func (f matchStringOnly) MatchString(pat, str string) (bool, error) { return f(pat, str) }

func (f matchStringOnly) StartCPUProfile(io.Writer) error { return errMain }

func (f matchStringOnly) StopCPUProfile() {}

func (f matchStringOnly) WriteProfileTo(string, io.Writer, int) error { return errMain }

func (f matchStringOnly) ImportPath() string { return "" }

func (f matchStringOnly) StartTestLog(io.Writer) {}

func (f matchStringOnly) StopTestLog() error { return errMain }

func (f matchStringOnly) SetPanicOnExit0(bool) {}

func (f matchStringOnly) CoordinateFuzzing(time.Duration, int64, time.Duration, int64, int, []corpusEntry, []reflect.Type, string, string) error {
	return nil
}

func (f matchStringOnly) RunFuzzWorker(func(corpusEntry) error) error { return nil }

func (f matchStringOnly) ReadCorpus(string, []reflect.Type) ([]corpusEntry, error) {
	return nil, nil
}

func (f matchStringOnly) CheckCorpus([]any, []reflect.Type) error { return nil }

func (f matchStringOnly) ResetCoverage()    {}
func (f matchStringOnly) SnapshotCoverage() {}

func (f matchStringOnly) InitRuntimeCoverage() (mode string, tearDown func(coverprofile string, gocoverdir string) (string, error), snapcov func() float64) {
	return "", func(string, string) (string, error) { return "", nil }, func() float64 { return 0 }
}
