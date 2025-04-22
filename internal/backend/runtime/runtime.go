// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package runtime implements connectors to various runtimes.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/xslices"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/runtime"
)

// Runtime is an abstraction for the data access.
type Runtime interface {
	Watch(context.Context, chan<- WatchResponse, ...QueryOption) error
	Get(context.Context, ...QueryOption) (any, error)
	List(context.Context, ...QueryOption) (ListResult, error)
	Create(context.Context, cosiresource.Resource, ...QueryOption) error
	Update(context.Context, cosiresource.Resource, ...QueryOption) error
	Delete(context.Context, ...QueryOption) error
}

// WatchResponse is a wrapper for the resources.WatchResponse.
type WatchResponse interface {
	ID() string
	Namespace() string
	Field(name string) (string, bool)
	Match(searchFor string) bool
	Unwrap() *resources.WatchResponse
}

// EventType returns event type for the response.
func EventType(resp WatchResponse) resources.EventType { return resp.Unwrap().Event.EventType }

// ListResult is a wrapper for the list result.
type ListResult struct {
	Items []runtime.ListItem
	Total int
}

// ListComparator is a comparator for the list items.
type ListComparator func(a, b runtime.ListItem) (int, error)

// SortInPlace sorts the list result.
func (l *ListResult) SortInPlace(cmp ListComparator) error { return unsafeSort(l.Items, cmp) }

// Slice returns a slice of the list result.
func (l *ListResult) Slice(offset, count int) ListResult {
	if offset >= len(l.Items) {
		return ListResult{
			Total: l.Total,
		}
	}

	items := l.Items[offset:]

	if count > 0 && count < len(items) {
		items = items[:count]
	}

	return ListResult{
		Items: items,
		Total: l.Total,
	}
}

// Filter filters Items using the provided predicate.
func (l *ListResult) Filter(match func(m runtime.ListItem) bool) ListResult {
	result := xslices.Filter(l.Items, match)

	return ListResult{
		Items: result,
		Total: len(result),
	}
}

// KubeconfigSource is implemented by runtimes that allow getting kubeconfigs.
type KubeconfigSource interface {
	GetKubeconfig(context.Context, *common.Context) (*rest.Config, error)
}

var (
	runtimeMu sync.RWMutex
	runtimes  = map[string]Runtime{}
)

// Install a runtime singleton for a type.
func Install(name string, runtime Runtime) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()

	runtimes[name] = &proxyRuntime{runtime}
}

// Get returns runtime for a type.
func Get(name string) (Runtime, error) { //nolint:ireturn
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()

	if runtime, ok := runtimes[name]; ok {
		return runtime, nil
	}

	return nil, fmt.Errorf("failed to find the runtime %v", name)
}

// LookupInterface looks for a specific implementation in runtimes.
func LookupInterface[T any](name string) (T, error) {
	var zero T

	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Interface {
		return zero, errors.New("can only be used with interface types")
	}

	if typ.NumMethod() != 1 {
		return zero, errors.New("can only be used with interfaces with a single method")
	}

	runtimeMu.RLock()
	defer runtimeMu.RUnlock()

	if runtime, ok := runtimes[name]; ok {
		res, ok := unwrap(runtime).(T)
		if !ok {
			return zero, fmt.Errorf("runtime with id %s is not %s", name, typeName(typ))
		}

		return res, nil
	}

	return zero, fmt.Errorf("failed to find the runtime %v", name)
}

func typeName(typ reflect.Type) string {
	if name := typ.Name(); name != "" {
		return name
	}

	return typ.String()
}

func unwrap(runtime Runtime) Runtime {
	for {
		wrapped, ok := runtime.(interface{ Unwrap() Runtime })
		if !ok {
			return runtime
		}

		runtime = wrapped.Unwrap()
	}
}

// NewBasicResponse creates a new basic response.
func NewBasicResponse(id string, namespace string, resp *resources.WatchResponse) BasicResponse {
	mustNotNil(resp, "nil response")
	mustNotNil(resp.Event, "nil response event")

	return BasicResponse{
		BasicItem: BasicItem[*resources.WatchResponse]{
			id: id,
			ns: namespace,
			v:  resp,
		},
	}
}

// BasicResponse is a basic implementation of the WatchResponse.
type BasicResponse struct {
	BasicItem[*resources.WatchResponse]
}

// Field implements WatchResponse. name can be "id", "namespace" or "event_type".
// If name is empty, it returns ID. If name is unknown, it returns false.
func (b *BasicResponse) Field(name string) (string, bool) {
	field, ok := b.BasicItem.Field(name)
	if ok {
		return field, true
	}

	if name == "event_type" {
		return string(EventType(b)), true
	}

	return "", false
}

// Match looks for a specific string in item data.
func (b *BasicResponse) Match(searchFor string) bool {
	return EventType(b) == resources.EventType_BOOTSTRAPPED || b.BasicItem.Match(searchFor)
}

// String implements fmt.Stringer.
func (b *BasicResponse) String() string {
	return fmt.Sprintf("{id=%q, namespace=%q, event_type=%q, total=%d}", b.id, b.ns, EventType(b), b.Unwrap().GetTotal())
}

// MakeBasicItem creates a new basic item.
func MakeBasicItem[T any](id string, ns string, v T) BasicItem[T] {
	return BasicItem[T]{id: id, ns: ns, v: v}
}

// BasicItem is a basic building block for the WatchResponse and ListItem.
type BasicItem[T any] struct {
	v  T
	id string
	ns string
}

// ID implements WatchResponse.
func (bi BasicItem[T]) ID() string { return bi.id }

// Namespace implements WatchResponse.
func (bi BasicItem[T]) Namespace() string { return bi.ns }

// Unwrap implements WatchResponse.
func (bi BasicItem[T]) Unwrap() T { return bi.v }

// Field implements WatchResponse. name can be "id", "namespace" or "event_type".
// If name is empty, it returns ID. If name is unknown, it returns false.
func (bi BasicItem[T]) Field(name string) (string, bool) {
	switch name {
	case "id", "":
		return bi.id, true
	case "namespace":
		return bi.ns, true
	default:
		return "", false
	}
}

// Match looks for a specific string in item data.
func (bi BasicItem[T]) Match(searchFor string) bool {
	return strings.Contains(bi.ID(), searchFor) ||
		strings.Contains(bi.Namespace(), searchFor)
}

// String implements fmt.Stringer.
func (bi BasicItem[T]) String() string {
	return fmt.Sprintf("{id=%q, namespace=%q}", bi.id, bi.ns)
}

func mustNotNil[T any](v *T, msg string) {
	if v == nil {
		panic(msg)
	}
}

// ResourceField returns a metadata field value.
func ResourceField(res cosiresource.Resource, name string) (string, bool) {
	if res == nil {
		return "", false
	}

	switch name {
	case "created":
		return timeToString(res.Metadata().Created()), true
	case "updated":
		return timeToString(res.Metadata().Updated()), true
	}

	fielder, ok := typed.LookupExtension[runtime.Fielder](res)
	if !ok {
		return "", false
	}

	return fielder.Field(name)
}

func timeToString(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05.000000000")
}

// MatchResource returns true if searchFor is in any field of metadata.
func MatchResource(res cosiresource.Resource, searchFor string) bool {
	if res == nil {
		return false
	}

	if strings.Contains(timeToString(res.Metadata().Created()), searchFor) ||
		strings.Contains(timeToString(res.Metadata().Updated()), searchFor) {
		return true
	}

	matcher, ok := typed.LookupExtension[runtime.Matcher](res)
	if !ok {
		return false
	}

	return matcher.Match(searchFor)
}
