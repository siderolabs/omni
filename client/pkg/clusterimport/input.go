// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clusterimport

import (
	"fmt"
	"io"
	"time"

	"github.com/blang/semver/v4"
)

// Input represents the input parameters required for importing a Talos cluster into Omni.
type Input struct {
	LogWriter                io.Writer
	ClusterID                string
	InitialTalosVersion      string
	TalosVersion             string
	InitialKubernetesVersion string
	KubernetesVersion        string
	BackupOutput             string
	Nodes                    []string
	Force                    bool
	DryRun                   bool
	SkipHealthCheck          bool
}

func (input *Input) getBackupOutput() string {
	backupFile := input.BackupOutput
	if backupFile == "" {
		backupFile = fmt.Sprintf("%s-backup-%s.zip", input.ClusterID, time.Now().Format("20060102-150405"))
	}

	return backupFile
}

func (input *Input) initVersions(nodeName, detectedTalosVersion, detectedKubernetesVersion string) error {
	talosVersion, err := determineVersion(input.TalosVersion, detectedTalosVersion, "talos", nodeName)
	if err != nil {
		return err
	}

	kubernetesVersion, err := determineVersion(input.KubernetesVersion, detectedKubernetesVersion, "kubernetes", nodeName)
	if err != nil {
		return err
	}

	input.TalosVersion = talosVersion.String()
	input.KubernetesVersion = kubernetesVersion.String()

	if input.InitialTalosVersion, err = determineInitialVersion(talosVersion, input.InitialTalosVersion, "talos"); err != nil {
		return err
	}

	if input.InitialKubernetesVersion, err = determineInitialVersion(kubernetesVersion, input.InitialKubernetesVersion, "kubernetes"); err != nil {
		return err
	}

	return nil
}

func determineVersion(inputVersion, detectedVersion string, component, nodeName string) (semver.Version, error) {
	parsedDetectedVersion, err := semver.ParseTolerant(detectedVersion)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse detected %s version %q from node %q: %w", component, detectedVersion, nodeName, err)
	}

	if inputVersion != "" {
		var parsedInputVersion semver.Version

		if parsedInputVersion, err = semver.ParseTolerant(inputVersion); err != nil {
			return semver.Version{}, fmt.Errorf("failed to parse requested %s version %q: %w", component, inputVersion, err)
		}

		if parsedInputVersion.NE(parsedDetectedVersion) {
			return semver.Version{}, fmt.Errorf("requested %s version %q does not match detected %s version %q", component, parsedInputVersion, component, parsedDetectedVersion)
		}
	}

	return parsedDetectedVersion, nil
}

func determineInitialVersion(version semver.Version, initialVersion, component string) (string, error) {
	if initialVersion == "" {
		return version.String(), nil
	}

	parsedInitialVersion, err := semver.ParseTolerant(initialVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse initial %s version %q: %w", component, initialVersion, err)
	}

	if parsedInitialVersion.GT(version) {
		return "", fmt.Errorf("initial %s version %q cannot be greater than detected version %q", component, parsedInitialVersion, version)
	}

	return parsedInitialVersion.String(), nil
}

func (input *Input) logf(line string, args ...any) {
	fmt.Fprintf(input.LogWriter, line+"\n", args...) //nolint:errcheck
}
