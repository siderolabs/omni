// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/pair/ordered"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/runtime"
)

type proxyRuntime struct{ Runtime }

func (p *proxyRuntime) Watch(ctx context.Context, responses chan<- WatchResponse, option ...QueryOption) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var group errgroup.Group

	opts := NewQueryOptions(option...)
	cmp := MakeWatchResponseComparator(opts.SortField, opts.SortDescending)
	ch := make(chan WatchResponse)
	produce := watchResponseProducer(responses, opts, cmp)

	group.Go(func() error {
		defer cancel()

		slc, err := takeSorted(ctx, ch, cmp)
		if err != nil {
			return err
		}

		for _, ev := range slc {
			if ok, err := produce(ctx, ev); !ok {
				return err
			}
		}

		for {
			select {
			case <-ctx.Done():
				return nil
			case ev := <-ch:
				if ok, err := produce(ctx, ev); !ok {
					return err
				}
			}
		}
	})

	group.Go(func() error {
		defer cancel()

		return p.Runtime.Watch(ctx, ch, option...)
	})

	return group.Wait()
}

func watchResponseProducer(
	responses chan<- WatchResponse,
	opts *QueryOptions,
	cmp WatchResponseComparator,
) func(ctx context.Context, wr WatchResponse) (bool, error) {
	offsetLimiter := MakeStreamOffsetLimiter(opts.Offset, opts.Limit, safeCmp(cmp, cmpNamespaceID[WatchResponse]))
	total := int32(0)

	return func(ctx context.Context, wr WatchResponse) (bool, error) {
		if !match(wr, opts.SearchFor) {
			return true, nil
		}

		wr.Unwrap().Total = changeTotal(wr, &total)

		if wr.Namespace() != "" && wr.ID() != "" {
			if !offsetLimiter.Check(wr) {
				return true, nil
			}
		}

		err := fill(wr, opts.SortField, opts.SortDescending)
		if err != nil {
			return false, err
		}

		if !channel.SendWithContext(ctx, responses, wr) {
			return false, nil
		}

		return true, nil
	}
}

func match(ev runtime.Matcher, searchFor []string) bool {
	return len(searchFor) == 0 ||
		slices.IndexFunc(searchFor, func(searchFor string) bool { return ev.Match(searchFor) }) != -1
}

func fill(r WatchResponse, field string, desc bool) error {
	if r.Namespace() == "" || r.ID() == "" {
		return nil
	}

	fieldData, err := getField(r, field)
	if err != nil {
		return err
	}

	// Mutating things is not a good idea, but we have to do it here.
	// In futre - make WatchResponse an internal type, and convert it to grpc in outer layer.
	u := r.Unwrap()
	u.SortFieldData = fieldData
	u.SortDescending = desc

	return nil
}

func changeTotal(ev WatchResponse, total *int32) int32 {
	switch EventType(ev) { //nolint:exhaustive
	case resources.EventType_CREATED:
		*total++
	case resources.EventType_DESTROYED:
		*total--
	}

	return *total
}

func takeSorted(ctx context.Context, ch chan WatchResponse, cmp WatchResponseComparator) ([]WatchResponse, error) {
	slc, ok := takeUntil(ctx, ch, func(ev WatchResponse) bool { return EventType(ev) == resources.EventType_BOOTSTRAPPED })
	if !ok {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, errors.New("failed to take data until BOOTSTRAPPED event")
	}

	err := SortResponses(slc, cmp)
	if err != nil {
		return nil, err
	}

	return slc, nil
}

func (p *proxyRuntime) List(ctx context.Context, option ...QueryOption) (ListResult, error) {
	res, err := p.Runtime.List(ctx, option...)
	if err != nil {
		return ListResult{}, err
	}

	opts := NewQueryOptions(option...)
	cmp := MakeFieldComparator(opts.SortField, opts.SortDescending, getField, cmpNamespaceID[fielder])

	res = res.Filter(func(m runtime.ListItem) bool { return match(m, opts.SearchFor) })

	err = res.SortInPlace(func(a, b runtime.ListItem) (int, error) { return cmp(a, b) })
	if err != nil {
		return ListResult{}, err
	}

	res = res.Slice(opts.Offset, opts.Limit)

	return res, nil
}

// Unwrap returns the underlying runtime.
func (p *proxyRuntime) Unwrap() Runtime {
	return p.Runtime
}

func takeUntil[T any](ctx context.Context, ch <-chan T, f func(v T) bool) ([]T, bool) {
	var res []T

	for {
		select {
		case <-ctx.Done():
			return res, false
		case v, ok := <-ch:
			if !ok {
				return res, false
			}

			res = append(res, v)

			if f(v) {
				return res, true
			}
		}
	}
}

// SortResponses sorts the slice of WatchResponse in a safe way.
func SortResponses(slc []WatchResponse, cmp WatchResponseComparator) error {
	return unsafeSort(slc, cmp)
}

// WatchResponseComparator is a comparator for WatchResponse.
type WatchResponseComparator func(a, b WatchResponse) (int, error)

// MakeWatchResponseComparator returns a comparator for WatchResponse.
func MakeWatchResponseComparator(field string, descending bool) WatchResponseComparator {
	if field == "" {
		field = "id"
	}

	cmp := MakeFieldComparator(field, descending, getField, cmpNamespaceID[fielder])

	return func(a, b WatchResponse) (int, error) {
		// BOOTSTRAPPED event should always be the last.
		switch pair.MakePair(EventType(a) == resources.EventType_BOOTSTRAPPED, EventType(b) == resources.EventType_BOOTSTRAPPED) {
		case pair.MakePair(true, false):
			return +1, nil
		case pair.MakePair(false, true):
			return -1, nil
		case pair.MakePair(true, true):
			return 0, nil
		}

		return cmp(a, b)
	}
}

type customError struct{ error }

func unsafeSort[T any](slc []T, cmp func(a, b T) (int, error)) (err error) {
	if len(slc) == 0 {
		return nil
	}

	if len(slc) == 1 {
		// Compare it with itself to check if it's possible to compare.
		_, err = cmp(slc[0], slc[0])
		if err != nil {
			return err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			if pnc, ok := r.(*customError); ok {
				err = pnc

				return
			}

			panic(err)
		}
	}()

	slices.SortFunc(slc, func(a, b T) int {
		res, err := cmp(a, b)
		if err != nil {
			panic(&customError{err})
		}

		return res
	})

	return nil
}

type fielder interface {
	idNamespace
	Field(string) (string, bool)
}

func getField(wr fielder, field string) (string, error) {
	res, ok := wr.Field(field)
	if !ok {
		return "", fmt.Errorf("failed to sort: field %q for element %q not found", field, wr.ID())
	}

	return res, nil
}

func safeCmp[T any](unsafeCmp func(a, b T) (int, error), cmp func(a, b T) int) func(a, b T) int {
	return func(a, b T) (result int) {
		res, err := unsafeCmp(a, b)
		if err != nil {
			return cmp(a, b)
		}

		return res
	}
}

type idNamespace interface {
	ID() string
	Namespace() string
}

func cmpNamespaceID[T idNamespace](a, b T) int {
	left := ordered.MakePair(a.Namespace(), a.ID())
	right := ordered.MakePair(b.Namespace(), b.ID())

	return left.Compare(right)
}

// MakeFieldComparator returns a comparator for the given field.
func MakeFieldComparator[T any](
	field string,
	descending bool,
	fieldExtractor func(T, string) (string, error),
	defaultCmp func(T, T) int,
) func(T, T) (int, error) {
	cmp := func(a, b T) (int, error) {
		if field == "" {
			return defaultCmp(a, b), nil
		}

		left, err := fieldExtractor(a, field)
		if err != nil {
			return 0, err
		}

		right, err := fieldExtractor(b, field)
		if err != nil {
			return 0, err
		}

		if res := strings.Compare(left, right); res != 0 {
			if descending {
				return -res, nil
			}

			return res, nil
		}

		return defaultCmp(a, b), nil
	}

	return cmp
}
