// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import "time"

// Storage defines Omni COSI state storage configuration.
type Storage struct {
	Default *StorageDefault `yaml:"default" validate:"required"`
	// Vault configuration where the state encryption keys are present.
	Vault Vault `yaml:"vault"`
	// Secondary storage is used to store the metrics and any frequently changed resources
	// which might overflow etcd resource history.
	Secondary BoltDB `yaml:"secondary" validate:"required"`
	// Default is the storage used for the default resource namespace in Omni.
}

// Vault allows setting vault configuration through the config file.
type Vault struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// StorageDefault defines storage configs.
type StorageDefault struct {
	// Kind can be either 'boltdb' or 'etcd'.
	Kind   string     `yaml:"kind" validate:"oneof=etcd boltdb"`
	Boltdb BoltDB     `yaml:"boltdb"`
	Etcd   EtcdParams `yaml:"etcd"`
}

// BoltDB defines boltdb storage configs.
type BoltDB struct {
	Path string `yaml:"path"`
}

// EtcdParams defines etcd storage configs.
type EtcdParams struct { ///nolint:govet
	// External etcd: list of endpoints, as host:port pairs.
	Endpoints            []string      `yaml:"endpoints" merge:"replace"`
	DialKeepAliveTime    time.Duration `yaml:"dialKeepAliveTime"`
	DialKeepAliveTimeout time.Duration `yaml:"dialKeepAliveTimeout"`
	CAFile               string        `yaml:"caFile"`
	CertFile             string        `yaml:"certFile"`
	KeyFile              string        `yaml:"keyFile"`

	// Use embedded etcd server (no clustering).
	Embedded            bool   `yaml:"embedded"`
	EmbeddedDBPath      string `yaml:"embeddedDBPath"`
	EmbeddedUnsafeFsync bool   `yaml:"embeddedUnsafeFsync"`
	// Force running elections.
	// Disabling elections is not possible if etcd is not running in embedded mode.
	RunElections bool `yaml:"runElections"`

	PrivateKeySource string   `yaml:"privateKeySource" validate:"required"`
	PublicKeyFiles   []string `yaml:"publicKeyFiles" merge:"replace"`
}
