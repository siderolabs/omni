// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kubernetes implements the connector that can pull data from the Kubernetes control plane.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net"
	goruntime "runtime"
	"strings"
	"text/template"
	"time"

	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/channel"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/connrotation"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	pkgruntime "github.com/siderolabs/omni/client/pkg/runtime"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/helpers"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Name kubernetes runtime string id.
var Name = common.Runtime_Kubernetes.String()

const (
	// KubernetesStatus controller anyways holds a reference to kubernetes clients
	// for all healthy (connected) clusters, so the cache size should be bigger to accommodate all of them
	// anyways to make hit rate higher.
	//
	// So we keep the number high enough here, as lowering it won't save memory if KubernetesStatus controller
	// anyways holds a reference to all of them.
	kubernetesClientLRUSize = 10240
	kubernetesClientTTL     = time.Hour
)

// New creates new Runtime.
func New(omniState state.State) (*Runtime, error) {
	return NewWithTTL(omniState, kubernetesClientTTL)
}

// NewWithTTL creates new Runtime with custom TTL.
func NewWithTTL(omniState state.State, ttl time.Duration) (*Runtime, error) {
	return &Runtime{
		clientsCache: expirable.NewLRU[string, *Client](kubernetesClientLRUSize, nil, ttl),

		state: omniState,

		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_k8s_clientfactory_cache_size",
			Help: "Number of Kubernetes clients in the cache of Kubernetes runtime.",
		}),
		metricActiveClients: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_k8s_clientfactory_active_clients",
			Help: "Number of active Kubernetes clients created by Kubernetes runtime.",
		}),
		metricCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_k8s_clientfactory_cache_hits_total",
			Help: "Number of Kubernetes client factory cache hits.",
		}),
		metricCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_k8s_clientfactory_cache_misses_total",
			Help: "Number of Kubernetes client factory cache misses.",
		}),
	}, nil
}

// Runtime implements runtime.Runtime.
type Runtime struct { //nolint:govet // there is just a single instance
	cleanuperTracker

	clientsCache *expirable.LRU[string, *Client]
	sf           singleflight.Group

	state state.State

	metricCacheSize, metricActiveClients prometheus.Gauge
	metricCacheHits, metricCacheMisses   prometheus.Counter
}

// Watch creates new kubernetes watch.
func (r *Runtime) Watch(ctx context.Context, events chan<- runtime.WatchResponse, setters ...runtime.QueryOption) error {
	opts := runtime.NewQueryOptions(setters...)

	selector := ""
	if len(opts.LabelSelectors) > 0 {
		selector = opts.LabelSelectors[0]
	}

	w := &Watch{
		resource: &watchRequest{
			Namespace: opts.Namespace,
			Type:      opts.Resource,
			ID:        opts.Name,
		},
		events:   events,
		selector: selector,
	}

	client, err := r.getOrCreateClient(ctx, opts)
	if err != nil {
		return err
	}

	if err := w.run(ctx, client); err != nil {
		return fmt.Errorf("error running watch: %w", err)
	}

	return nil
}

// Get implements runtime.Runtime.
func (r *Runtime) Get(ctx context.Context, setters ...runtime.QueryOption) (any, error) {
	opts := runtime.NewQueryOptions(setters...)

	client, err := r.getOrCreateClient(ctx, opts)
	if err != nil {
		return nil, err
	}

	res, err := client.Get(ctx, opts.Resource, opts.Name, opts.Namespace, metav1.GetOptions{})
	if err != nil {
		return nil, wrapError(err, opts)
	}

	return res, nil
}

// List implements runtime.Runtime.
func (r *Runtime) List(ctx context.Context, setters ...runtime.QueryOption) (runtime.ListResult, error) {
	opts := runtime.NewQueryOptions(setters...)

	client, err := r.getOrCreateClient(ctx, opts)
	if err != nil {
		return runtime.ListResult{}, err
	}

	listOpts := metav1.ListOptions{
		FieldSelector: opts.FieldSelector,
	}

	if len(opts.LabelSelectors) > 0 {
		listOpts.LabelSelector = opts.LabelSelectors[0]
	}

	list, err := client.List(ctx, opts.Resource, opts.Namespace, listOpts)
	if err != nil {
		return runtime.ListResult{}, wrapError(err, opts)
	}

	res := make([]pkgruntime.ListItem, 0, len(list.Items))

	for _, obj := range list.Items {
		res = append(res, newItem(obj))
	}

	return runtime.ListResult{
		Items: res,
		Total: len(res),
	}, nil
}

// Create implements runtime.Runtime.
func (r *Runtime) Create(ctx context.Context, resource cosiresource.Resource, setters ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// Update implements runtime.Runtime.
func (r *Runtime) Update(ctx context.Context, resource cosiresource.Resource, setters ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// Delete implements runtime.Runtime.
func (r *Runtime) Delete(ctx context.Context, setters ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// DestroyClient closes Kubernetes client and deletes it from the client cache.
func (r *Runtime) DestroyClient(id string) {
	r.clientsCache.Remove(id)
	r.sf.Forget(id)

	r.triggerCleanupers(id)
}

var oidcKubeConfigTemplate = strings.TrimSpace(`
apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: {{ .KubernetesProxyEndpoint }}
    name: {{ .InstanceName }}-{{ .ClusterName }}
contexts:
  - context:
      cluster: {{ .InstanceName }}-{{ .ClusterName }}
      namespace: default
      user: {{ .InstanceName }}-{{ .ClusterName }}{{- if .Identity }}-{{ .Identity }}{{ end }}
    name: {{ .InstanceName }}-{{ .ClusterName }}
current-context: {{ .InstanceName }}-{{ .ClusterName }}
users:
- name: {{ .InstanceName }}-{{ .ClusterName }}{{- if .Identity }}-{{ .Identity }}{{ end }}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
        - oidc-login
        - get-token
        - --oidc-issuer-url={{ .EndpointOIDC }}
        - --oidc-client-id={{ .ClientID }}
        - --oidc-extra-scope=cluster:{{ .ClusterName }}
        {{- range $option := .ExtraOptions }}
        - --{{ $option }}
        {{- end }}
      command: kubectl
      env: null
      provideClusterInfo: false
`) + "\n"

// GetOIDCKubeconfig generates kubeconfig for the user using Omni as OIDC Proxy.
func (r *Runtime) GetOIDCKubeconfig(context *common.Context, identity string, extraOptions ...string) ([]byte, error) {
	tmpl, err := template.New("kubeconfig").Parse(oidcKubeConfigTemplate)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	issuerEndpoint, err := config.Config.GetOIDCIssuerEndpoint()
	if err != nil {
		return nil, err
	}

	if err = tmpl.Execute(&buf, struct {
		InstanceName string
		ClusterName  string

		KubernetesProxyEndpoint string
		EndpointOIDC            string

		ClientID     string
		Identity     string
		ExtraOptions []string
	}{
		InstanceName: config.Config.Account.Name,
		ClusterName:  context.Name,

		EndpointOIDC:            issuerEndpoint,
		KubernetesProxyEndpoint: config.Config.Services.KubernetesProxy.URL(),

		ClientID:     external.DefaultClientID,
		Identity:     identity,
		ExtraOptions: extraOptions,
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// BreakGlassKubeconfig returns raw kubeconfig.
func (r *Runtime) BreakGlassKubeconfig(ctx context.Context, id string) ([]byte, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	kubeconfig, err := safe.ReaderGetByID[*omni.Kubeconfig](ctx, r.state, id)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.Load(kubeconfig.TypedSpec().Value.Data)
	if err != nil {
		return nil, err
	}

	endpoints, err := helpers.GetMachineEndpoints(ctx, r.state, id)
	if err != nil {
		return nil, err
	}

	if len(endpoints) > 0 {
		for _, c := range config.Clusters {
			c.Server = fmt.Sprintf("https://%s", net.JoinHostPort(endpoints[0], "6443"))
		}
	}

	return clientcmd.Write(*config)
}

// GetKubeconfig returns kubeconfig.
func (r *Runtime) GetKubeconfig(ctx context.Context, context *common.Context) (*rest.Config, error) {
	if context == nil {
		return nil, stderrors.New("the context is required")
	}

	ctx = actor.MarkContextAsInternalActor(ctx)
	id := context.Name

	kubeconfig, err := safe.ReaderGetByID[*omni.Kubeconfig](ctx, r.state, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, fmt.Errorf("kubeconfig for cluster %s is not found", id)
		}

		return nil, err
	}

	return clientcmd.RESTConfigFromKubeConfig(kubeconfig.TypedSpec().Value.Data)
}

// GetClient returns K8s client.
func (r *Runtime) GetClient(ctx context.Context, cluster string) (*Client, error) {
	// require proper auth to access kubeconfig, but allow internal access
	// this method gets called from the Kubernetes backend
	if !actor.ContextIsInternalActor(ctx) {
		if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
			return nil, err
		}
	}

	return r.getOrCreateClient(ctx, &runtime.QueryOptions{
		Context: cluster,
	})
}

func (r *Runtime) getOrCreateClient(ctx context.Context, opts *runtime.QueryOptions) (*Client, error) {
	id := opts.Context

	if client, ok := r.clientsCache.Get(id); ok {
		r.metricCacheHits.Inc()

		return client, nil
	}

	ch := r.sf.DoChan(id, func() (any, error) {
		cfg, err := r.GetKubeconfig(ctx, &common.Context{Name: id})
		if err != nil {
			return nil, err
		}

		client, err := NewClient(cfg)
		if err != nil {
			return nil, err
		}

		r.metricActiveClients.Inc()
		r.metricCacheMisses.Inc()

		goruntime.AddCleanup(client, func(m prometheus.Gauge) { m.Dec() }, r.metricActiveClients)
		goruntime.AddCleanup(client, func(dialer *connrotation.Dialer) { dialer.CloseAll() }, client.dialer)

		r.clientsCache.Add(id, client)

		return client, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		return res.Val.(*Client), nil //nolint:forcetypeassert,errcheck // we know the type
	}
}

// Describe implements prom.Collector interface.
func (r *Runtime) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(r, ch)
}

// Collect implements prom.Collector interface.
func (r *Runtime) Collect(ch chan<- prometheus.Metric) {
	r.metricActiveClients.Collect(ch)

	r.metricCacheSize.Set(float64(r.clientsCache.Len()))
	r.metricCacheSize.Collect(ch)

	r.metricCacheHits.Collect(ch)
	r.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &Runtime{}

// Watch watches kubernetes resources.
type Watch struct {
	events   chan<- runtime.WatchResponse
	resource *watchRequest
	selector string
}

// watchRequest describes target for Watch.
type watchRequest struct {
	Namespace, Type, ID string
}

func (w *Watch) run(ctx context.Context, client *Client) error {
	namespace := v1.NamespaceAll
	if w.resource.Namespace != "" {
		namespace = w.resource.Namespace
	}

	filter := func(options *metav1.ListOptions) {
		if w.selector != "" {
			options.LabelSelector = w.selector
		}

		if w.resource.ID != "" {
			options.FieldSelector = fmt.Sprintf("metadata.name=%s", w.resource.ID)
		}
	}

	dynamicInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client.Dynamic(), 0, namespace, filter)

	gvr, err := client.getGVR(w.resource.Type)
	if err != nil {
		return err
	}

	gvk, err := client.kindFor(*gvr)
	if err != nil {
		return err
	}

	if err = filterGVK(gvk); err != nil {
		return err
	}

	informer := dynamicInformer.ForResource(*gvr)

	errCh := make(chan error, 1)

	eg, ctx := panichandler.ErrGroupWithContext(ctx)

	if err := informer.Informer().SetWatchErrorHandler(func(_ *toolscache.Reflector, e error) {
		channel.SendWithContext(ctx, errCh, e)
	}); err != nil {
		return err
	}

	informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{ //nolint:errcheck
		AddFunc: func(obj any) {
			event, err := runtime.NewWatchResponse(resources.EventType_CREATED, obj, nil)
			if err != nil {
				channel.SendWithContext(ctx, errCh, err)

				return
			}

			channel.SendWithContext(ctx, w.events, respFromK8sObject(event, obj))
		},
		UpdateFunc: func(oldObj, newObj any) {
			event, err := runtime.NewWatchResponse(resources.EventType_UPDATED, newObj, oldObj)
			if err != nil {
				channel.SendWithContext(ctx, errCh, err)

				return
			}

			channel.SendWithContext(ctx, w.events, respFromK8sObject(event, newObj))
		},
		DeleteFunc: func(obj any) {
			event, err := runtime.NewWatchResponse(resources.EventType_DESTROYED, obj, nil)
			if err != nil {
				channel.SendWithContext(ctx, errCh, err)

				return
			}

			channel.SendWithContext(ctx, w.events, respFromK8sObject(event, obj))
		},
	})

	eg.Go(func() error {
		dynamicInformer.Start(ctx.Done())

		return nil
	})

	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			return err
		}
	})

	dynamicInformer.WaitForCacheSync(ctx.Done())

	channel.SendWithContext(ctx, w.events, booststrappedResponse())

	return eg.Wait()
}

func wrapError(err error, opts *runtime.QueryOptions) error {
	md := cosiresource.NewMetadata(opts.Namespace, opts.Resource, opts.Name, cosiresource.VersionUndefined)

	switch {
	case errors.IsConflict(err):
		fallthrough
	case errors.IsAlreadyExists(err):
		return inmem.ErrAlreadyExists(md)
	case errors.IsNotFound(err):
		return inmem.ErrNotFound(md)
	}

	return err
}

// UnstructuredFromResource creates Unstructured resource from k8s.KubernetesResource.
func UnstructuredFromResource(resource cosiresource.Resource) (*unstructured.Unstructured, error) {
	if resource.Metadata().Type() != k8s.KubernetesResourceType {
		return nil, fmt.Errorf("kubernetes runtime accepts only %s type as an input, got %s", k8s.KubernetesResourceType, resource.Metadata().Type())
	}

	var (
		res *k8s.KubernetesResource
		ok  bool
	)

	if res, ok = resource.(*k8s.KubernetesResource); !ok {
		return nil, stderrors.New("failed to convert spec to KubernetesResourceSpec")
	}

	data := res.TypedSpec()

	if data == nil {
		return nil, stderrors.New("no resource spec is defined in the resource")
	}

	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(*data), &obj.Object); err != nil {
		return nil, err
	}

	return obj, nil
}

func respFromK8sObject(resp *resources.WatchResponse, obj any) runtime.WatchResponse {
	// copied from klog.KMetadata
	type metadata interface {
		GetName() string
		GetNamespace() string
	}

	md, ok := obj.(metadata)
	if !ok {
		return newWatchResponse("", "", resp)
	}

	return newWatchResponse(md.GetName(), md.GetNamespace(), resp)
}

func booststrappedResponse() runtime.WatchResponse {
	return newWatchResponse(
		"",
		"",
		&resources.WatchResponse{
			Event: &resources.Event{EventType: resources.EventType_BOOTSTRAPPED},
		},
	)
}

func newWatchResponse(id, namespace string, resp *resources.WatchResponse) runtime.WatchResponse {
	return &watchResponse{BasicResponse: runtime.NewBasicResponse(id, namespace, resp)}
}

type watchResponse struct {
	runtime.BasicResponse
}

func newItem(obj unstructured.Unstructured) pkgruntime.ListItem {
	return &item{BasicItem: runtime.MakeBasicItem(obj.GetName(), obj.GetNamespace(), obj)}
}

type item struct {
	runtime.BasicItem[unstructured.Unstructured]
}

func (it *item) Field(name string) (string, bool) {
	val, ok := it.BasicItem.Field(name)
	if ok {
		return val, true
	}

	// TODO: add field search here
	return "", false
}

func (it *item) Match(searchFor string) bool {
	return it.BasicItem.Match(searchFor)
}

func (it *item) Unwrap() any {
	return it.BasicItem.Unwrap()
}
