// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// generate-certs is used to generate local CA certs for development and docker-compose.override.yml file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/siderolabs/gen/ensure"
	"gopkg.in/yaml.v3"
)

var cfgFile = flag.String("config", "", "config file path")

func main() {
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		printStderr("usage: %s [install|uninstall|generate]", os.Args[0])
		os.Exit(1)
	}

	toRun := install

	switch args[0] {
	case "install":
		break
	case "uninstall":
		toRun = uninstall
	case "generate":
		toRun = generate
	default:
		printStderr("usage: %s [install|uninstall|generate]", os.Args[0])
		os.Exit(1)
	}

	run := func() error {
		if ok, err := fileExist("./hack/compose/docker-compose.yml"); err != nil {
			return fmt.Errorf("error checking for docker-compose.yml: %w", err)
		} else if !ok {
			return errors.New("must be run from the root of the repo")
		}

		return toRun()
	}
	if err := run(); err != nil {
		printStderr("\n\napp failed with error: %v\n", err)
		os.Exit(1)
	}
}

func install() (err error) {
	defer handleErr(&err)

	certsDir := getCARootDir()
	ensure.NoError(os.MkdirAll(certsDir, 0o755))
	ensure.Value(exec.LookPath("mkcert"))
	ensure.NoError(os.Setenv("CAROOT", certsDir))
	ensure.NoError(runApp("mkcert", "-install"))

	printStderr("generated certs in %s\n", certsDir)

	return nil
}

func uninstall() (err error) {
	defer handleErr(&err)

	ensure.Value(exec.LookPath("mkcert"))

	if certsDir := getCARootDir(); ensure.Value(dirExists(certsDir)) {
		printStderr("found certs dir '%s'\n\n", certsDir)
		ensure.NoError(os.Setenv("CAROOT", certsDir))
	}

	ensure.NoError(runApp("mkcert", "-uninstall"))

	return nil
}

func generate() (err error) {
	defer handleErr(&err)

	if *cfgFile == "" {
		return errors.New("config file path is required")
	}

	certsDir := getCARootDir()
	if !ensure.Value(dirExists(certsDir)) {
		return fmt.Errorf("no certs dir found at '%s', please run 'install' before", certsDir)
	}

	var cfg cfg

	ensure.NoError(yaml.Unmarshal(ensure.Value(os.ReadFile(*cfgFile)), &cfg))
	ensure.NoError(os.Setenv("CAROOT", certsDir))
	ensure.NoError(os.MkdirAll("./hack/generate-certs/certs", 0o755))

	mkcertArgs := []string{
		"-cert-file",
		"./hack/generate-certs/certs/localhost.pem",
		"-key-file",
		"./hack/generate-certs/certs/localhost-key.pem",
		cfg.Host,
	}

	mkcertArgs = append(mkcertArgs, cfg.AdditionalHosts...)

	ensure.NoError(runApp("mkcert", mkcertArgs...))

	file := ensure.Value(os.OpenFile("./hack/compose/docker-compose.override.yml", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644))

	_, port, err := net.SplitHostPort(cfg.BindAddr)
	if err != nil {
		return fmt.Errorf("error parsing bind addr: %w", err)
	}

	if port == "443" {
		port = ""
	}

	data := struct {
		ClientID         string
		Auth0Domain      string
		Host             string
		Port             string
		Email            string
		BindAddr         string
		RegistryMirrors  map[string]string
		PprofBindAddress string
	}{
		ClientID:         cfg.ClientID,
		Auth0Domain:      cfg.Auth0Domain,
		Host:             cfg.Host,
		Port:             port,
		Email:            cfg.Email,
		BindAddr:         cfg.BindAddr,
		RegistryMirrors:  cfg.RegistryMirrors,
		PprofBindAddress: cfg.PprofBindAddress,
	}

	ensure.NoError(template.Must(template.New("docker-compose.override.yml").Parse(composeTemplate)).Execute(file, &data))

	return nil
}

func runApp(app string, args ...string) error {
	cmd := exec.Command(app, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running '%s': %w", app, err)
	}

	return nil
}

func dirExists(path string) (bool, error) {
	s, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return s.IsDir(), nil
}

func fileExist(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func getCARootDir() string {
	return filepath.Join(ensure.Value(os.Getwd()), "hack/generate-certs/ca-root")
}

type expectedError struct{ error }

func handleErr(err *error) {
	if r := recover(); r != nil {
		if e, ok := r.(*expectedError); ok {
			*err = e.error
		} else {
			panic(r)
		}
	}
}

func printStderr(msg string, args ...any) {
	ensure.Value(fmt.Fprintf(os.Stderr, msg, args...))
}

type cfg struct { //nolint:govet
	Email            string            `yaml:"email"`
	Host             string            `yaml:"host"`
	AdditionalHosts  []string          `yaml:"additional-hosts"`
	BindAddr         string            `yaml:"bind_addr"`
	ClientID         string            `yaml:"client-id"`
	Auth0Domain      string            `yaml:"auth0-domain"`
	RegistryMirrors  map[string]string `yaml:"registry-mirrors"`
	PprofBindAddress string            `yaml:"pprof-bind-address"`
}

func (c *cfg) UnmarshalYAML(value *yaml.Node) error {
	type innerCfg cfg

	err := value.Decode((*innerCfg)(c))
	if err != nil {
		return err
	}

	return c.validate()
}

func (c *cfg) validate() error {
	switch {
	case c.Email == "":
		return errors.New("email is required")
	case c.Host == "":
		return errors.New("host is required")
	case c.BindAddr == "":
		return errors.New("bind_addr is required")
	case c.ClientID == "":
		return errors.New("client-id is required")
	case c.Auth0Domain == "":
		return errors.New("auth0-domain is required")
	default:
		return nil
	}
}

const composeTemplate = `version: '3.8'
services:
  omni:
    command: >-
      --siderolink-wireguard-advertised-addr 172.20.0.1:50180
      --siderolink-wireguard-bind-addr='0.0.0.0:50180'
      --private-key-source vault://secret/omni-private-key
      --auth-auth0-enabled true
      --auth-auth0-client-id {{ .ClientID }}
      --auth-auth0-domain {{ .Auth0Domain }}
      --initial-users {{ .Email }},test-user@siderolabs.com
      --advertised-api-url https://{{ .Host }}{{- if .Port -}}:{{- end -}}{{ .Port }}
      --advertised-kubernetes-proxy-url https://{{ .Host }}:8095/
      --workload-proxying-enabled=true
      --key /etc/ssl/omni-certs/localhost-key.pem
      --cert /etc/ssl/omni-certs/localhost.pem
      --bind-addr {{ .BindAddr }}
      --frontend-dst http://127.0.0.1:8121
      --frontend-bind 0.0.0.0:8120
      --debug
      --etcd-embedded-unsafe-fsync=true
      --etcd-backup-s3
      --audit-log-dir /tmp/omni-data/audit-logs
      {{- range $key, $value := .RegistryMirrors }}
      --registry-mirror {{ $key }}={{ $value }}
      {{- end }}
      {{- if ne .PprofBindAddress ""}}
      --pprof-bind-addr {{ .PprofBindAddress }}
      {{- end}}
	omni-inspector:
		environment:
			- OMNI_ENDPOINT={{ .BindAddr }}
`
