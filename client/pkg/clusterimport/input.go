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
	LogWriter       io.Writer
	Versions        Versions
	BackupOutput    string
	Nodes           []string
	Force           bool
	DryRun          bool
	SkipHealthCheck bool
}

type Versions struct {
	InitialTalosVersion      string
	TalosVersion             string
	InitialKubernetesVersion string
	KubernetesVersion        string
}

func (input *Input) getBackupOutput(clusterID string) string {
	backupFile := input.BackupOutput
	if backupFile == "" {
		backupFile = fmt.Sprintf("%s-backup-%s.zip", clusterID, time.Now().Format("20060102-150405"))
	}

	return backupFile
}

func (input *Input) initVersions(nodeName, detectedTalosVersion, detectedKubernetesVersion string) (Versions, error) {
	talosVersion, err := input.determineVersion(input.Versions.TalosVersion, detectedTalosVersion, "talos", nodeName)
	if err != nil {
		return Versions{}, err
	}

	kubernetesVersion, err := input.determineVersion(input.Versions.KubernetesVersion, detectedKubernetesVersion, "kubernetes", nodeName)
	if err != nil {
		return Versions{}, err
	}

	versions := Versions{
		TalosVersion:      talosVersion.String(),
		KubernetesVersion: kubernetesVersion.String(),
	}

	if versions.InitialTalosVersion, err = input.determineInitialVersion(talosVersion, versions.InitialTalosVersion, "talos", nodeName); err != nil {
		return Versions{}, err
	}

	if versions.InitialKubernetesVersion, err = input.determineInitialVersion(kubernetesVersion, versions.InitialKubernetesVersion, "kubernetes", nodeName); err != nil {
		return Versions{}, err
	}

	return versions, nil
}

func (input *Input) determineVersion(inputVersion, detectedVersion string, component, nodeName string) (semver.Version, error) {
	parsedDetectedVersion, err := semver.ParseTolerant(detectedVersion)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse detected %s version %q from node %q: %w", component, detectedVersion, nodeName, err)
	}

	if inputVersion == "" {
		input.logf("%s version has not been provided, using detected version %s from node %s", component, parsedDetectedVersion, nodeName)

		return parsedDetectedVersion, nil
	}

	var parsedInputVersion semver.Version

	if parsedInputVersion, err = semver.ParseTolerant(inputVersion); err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse requested %s version %q: %w", component, inputVersion, err)
	}

	if parsedInputVersion.NE(parsedDetectedVersion) {
		return semver.Version{}, fmt.Errorf("requested %s version %q does not match detected %s version %q", component, parsedInputVersion, component, parsedDetectedVersion)
	}

	return parsedDetectedVersion, nil
}

func (input *Input) determineInitialVersion(version semver.Version, initialVersion, component, nodeName string) (string, error) {
	if initialVersion == "" {
		input.logf("initial %s version has not been provided, using detected version %s from node %s", component, version, nodeName)

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
