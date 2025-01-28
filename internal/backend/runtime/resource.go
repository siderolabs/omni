// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type protobufWrapper interface {
	GetValue() proto.Message
}

// Version implements a special uint64 type which can be unmarshalled from COSI version that can be string if it's undefined.
type Version uint64

// UnmarshalYAML implements yaml.Unmarshaler.
func (v *Version) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == resource.VersionUndefined.String() {
		return nil
	}

	val, err := strconv.ParseUint(value.Value, 10, 64)
	if err != nil {
		return err
	}

	*v = Version(val)

	return nil
}

// Metadata wraps COSI or Talos metadata into a json serializable object.
type Metadata struct {
	Created     time.Time           `json:"created" yaml:"created"`
	Updated     time.Time           `json:"updated" yaml:"updated"`
	Namespace   resource.Namespace  `json:"namespace" yaml:"namespace"`
	Type        resource.Type       `json:"type" yaml:"type"`
	ID          resource.ID         `json:"id" yaml:"id"`
	Owner       resource.Owner      `json:"owner" yaml:"owner"`
	Phase       string              `json:"phase" yaml:"phase"`
	Node        string              `json:"node,omitempty" yaml:"node"`
	Labels      map[string]string   `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string   `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Finalizers  resource.Finalizers `json:"finalizers,omitempty" yaml:"finalizers,omitempty"`
	Version     Version             `json:"version" yaml:"version,omitempty"`
}

// Resource wraps Talos and COSI resource response to be encoded as JSON.
type Resource struct {
	Spec     any               `yaml:"spec" json:"spec"`
	Resource resource.Resource `yaml:"-" json:"-"`
	ID       string            `yaml:"-" json:"-"`
	Metadata Metadata          `yaml:"metadata" json:"metadata"`
}

// ResourceOptions describes additional resource wrapper options.
type ResourceOptions struct {
	metadata *common.Metadata
}

// ResourceOption is a single resource creation option.
type ResourceOption func(*ResourceOptions)

// WithMetadata creates resource with Talos metadata.
func WithMetadata(value *common.Metadata) ResourceOption {
	return func(o *ResourceOptions) {
		o.metadata = value
	}
}

// NewResource creates new resource.
func NewResource(r resource.Resource, options ...ResourceOption) (*Resource, error) {
	opts := &ResourceOptions{}
	for _, o := range options {
		o(opts)
	}

	s, err := resource.MarshalYAML(r)
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return nil, err
	}

	parts := make([]string, 0, 3)
	if opts.metadata != nil {
		parts = append(parts, opts.metadata.Hostname)
	}

	parts = append(parts, r.Metadata().Namespace(), r.Metadata().ID())

	res := &Resource{
		ID: strings.Join(parts, "/"),
	}
	if err = yaml.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	if opts.metadata != nil {
		res.Metadata.Node = opts.metadata.Hostname
	}

	if wrapped, ok := r.Spec().(protobufWrapper); ok {
		res.Spec = wrapped.GetValue()
	}

	res.Resource = r

	return res, nil
}

// protoMessage wraps proto.Message to leverage protojson encoder.
type protoMessage struct {
	Message proto.Message
}

func (r protoMessage) MarshalJSON() ([]byte, error) {
	data, err := MarshalJSON(r.Message)

	return []byte(data), err
}

// MarshalJSON overrides default marshal behavior replacing spec
// with the wrapped struct, that uses protojson encoder.
func (r *Resource) MarshalJSON() ([]byte, error) {
	m, ok := r.Spec.(proto.Message)
	if !ok {
		return json.Marshal(*r)
	}

	return json.Marshal(Resource{
		Spec:     protoMessage{Message: m},
		Metadata: r.Metadata,
		Resource: r.Resource,
		ID:       r.ID,
	})
}
