// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/connrotation"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Client wrapper.
type Client struct {
	client    dynamic.Interface
	clientset *kubernetes.Clientset
	Mapper    meta.RESTMapper
	dialer    *connrotation.Dialer
	closeCh   chan struct{}
	closeOnce sync.Once
}

// Resource ...
func (c *Client) Resource(res *unstructured.Unstructured) (dynamic.ResourceInterface, error) { //nolint:ireturn
	var versions []string

	gvk := res.GroupVersionKind()

	if gvk.Version != "" {
		versions = append(versions, gvk.Version)
	}

	mapping, err := c.Mapper.RESTMapping(gvk.GroupKind(), versions...)
	if err != nil {
		return nil, err
	}

	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = c.client.Resource(mapping.Resource).Namespace(res.GetNamespace())
	} else {
		dr = c.client.Resource(mapping.Resource)
	}

	return dr, nil
}

func filterGVK(gvk schema.GroupVersionKind) error {
	switch gvk.String() {
	case "/v1, Kind=Node":
	case "/v1, Kind=Pod":
	default:
		return status.Errorf(codes.Unimplemented, "unsupported resource kind %q", gvk.String())
	}

	return nil
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (c *Client) Get(ctx context.Context, resource, name, namespace string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	res, err := c.parseResource(resource, namespace)
	if err != nil {
		return nil, err
	}

	if err = filterGVK(res.GroupVersionKind()); err != nil {
		return nil, err
	}

	dr, err := c.Resource(res)
	if err != nil {
		return nil, err
	}

	return dr.Get(ctx, name, opts, subresources...)
}

// List retrieves the list of object for the given object type from the Kubernetes Cluster.
func (c *Client) List(ctx context.Context, resource, namespace string, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	res, err := c.parseResource(resource, namespace)
	if err != nil {
		return nil, err
	}

	if err = filterGVK(res.GroupVersionKind()); err != nil {
		return nil, err
	}

	dr, err := c.Resource(res)
	if err != nil {
		return nil, err
	}

	return dr.List(ctx, opts)
}

// Dynamic returns the underlying dynamic client.
func (c *Client) Dynamic() dynamic.Interface { //nolint:ireturn
	return c.client
}

// Clientset returns the underlying clientset.
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// Close closes all clients.
func (c *Client) Close() {
	c.dialer.CloseAll()
	c.closeOnce.Do(func() { close(c.closeCh) })
}

// Wait returns a channel that will be closed when the client is closed.
func (c *Client) Wait() <-chan struct{} {
	return c.closeCh
}

func (c *Client) kindFor(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return c.Mapper.KindFor(gvr)
}

func (c *Client) parseResource(resource, namespace string) (*unstructured.Unstructured, error) {
	gvr, err := c.getGVR(resource)
	if err != nil {
		return nil, err
	}

	res := &unstructured.Unstructured{}

	gvk, err := c.kindFor(*gvr)
	if err != nil {
		return nil, err
	}

	res.SetGroupVersionKind(gvk)
	res.SetNamespace(namespace)

	return res, nil
}

func (c *Client) getGVR(resource string) (*schema.GroupVersionResource, error) {
	var gvr *schema.GroupVersionResource

	parts := strings.Split(resource, ".")

	var err error

	switch {
	case len(parts) == 2:
		gvr = &schema.GroupVersionResource{
			Resource: parts[0],
			Version:  parts[1],
		}
	case len(parts) == 1:
		gvr, err = c.discoverGVR(resource)
		if err != nil {
			return nil, err
		}
	default:
		gvr, _ = schema.ParseResourceArg(resource)
	}

	if gvr == nil {
		return nil, errors.New("couldn't parse resource name")
	}

	return gvr, nil
}

func (c *Client) discoverGVR(resource string) (*schema.GroupVersionResource, error) {
	gvr := &schema.GroupVersionResource{
		Resource: resource,
	}

	resources, err := c.clientset.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		for _, r := range res.APIResources {
			if r.Name == resource {
				gv, err := schema.ParseGroupVersion(res.GroupVersion)
				if err != nil {
					return nil, err
				}

				gvr.Version = gv.Version
				gvr.Group = gv.Group
			}
		}
	}

	return gvr, nil
}

// NewClient creates new Kubernetes client.
func NewClient(config *rest.Config) (*Client, error) {
	dialerRef := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	dialer := connrotation.NewDialer(dialerRef.DialContext)
	config.Dial = dialer.DialContext

	client, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for %q: %w", config.Host, err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(config, client)
	if err != nil {
		return nil, err
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	c, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{c, clientset, mapper, dialer, make(chan struct{}), sync.Once{}}, nil
}
