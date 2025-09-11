// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './fixtures.js'

test('create cluster', async ({ page, loginUser }) => {
  await page.getByRole('link', { name: 'Clusters' }).click()
  await page.getByRole('button', { name: 'Create Cluster' }).click()

  await page.click('div.machine-set-label#CP >> nth=0')
  await page.click('div.machine-set-label#W0 >> nth=1')
  await page.click('button#CP')

  const editor = page.locator('div.monaco-editor').first()

  await editor.click()

  await editor.press('Control+a')
  await editor.press('Delete')
  await editor.pressSequentially(`machine:
 network:
   hostname: deadbeef`)

  await page.click('button:has-text("Save")')

  await page.locator('button#extensions-CP').first().click()

  await page.click('span:has-text("usb-modem-drivers")')

  await page.click('button:has-text("Save")')

  await page.click('button:has-text("Create Cluster")')

  await page.getByRole('heading', { name: 'talos-default' }).waitFor()

  await page.getByRole('link', { name: 'Clusters' }).click()

  await page.locator('#talos-default-cluster-box').waitFor()

  await page.click('div.clusters-grid a')

  await page.click('button[type="button"]:has-text("Cluster Scaling")')

  await page.click('div.machine-set-label#W0 >> nth=0')

  await page.click('button:has-text("Update")')

  // Wait for the scaling to navigate successfully to the cluster overview page to avoid a race condition where
  // the test navigates to the Clusters page too early, then the scaling succeeds and goes to the Cluster Overview page
  await page.locator('text=Updated Cluster Configuration').waitFor()

  await page.getByRole('link', { name: 'Clusters' }).click()

  await expect(async () => {
    await expect(page.locator('div.clusters-grid')).toHaveCount(1)

    const runningAndTotal = await page.locator('#machine-count').textContent()
    const [, machinesTotal] = runningAndTotal?.split('/') ?? ''

    await expect(page.locator('text=deadbeef')).not.toHaveCount(0)
    expect(machinesTotal).toBe('3')

    await expect(page.locator('div[id="machine-set-phase-name"]:has-text("Running")')).toHaveCount(
      2,
    )
    await expect(
      page.locator('div[id="cluster-machine-stage-name"]:has-text("Running")'),
    ).toHaveCount(3)
  }).toPass({
    intervals: [5_000],
    timeout: 900_000,
  })

  await page.click('text=deadbeef')

  await page.click('text=Extensions')

  await page.locator('text=siderolabs/usb-modem-drivers').waitFor()

  await expect(page.locator('text=siderolabs/usb-modem-drivers')).toHaveCount(1)

  /////////assertTemplateExportAndSync

  // ctx, cancel := context.WithTimeout(s.T().Context(), 60*time.Second)
  // s.T().Cleanup(cancel)

  // s.prepareOmnictl(ctx)

  // templatePath := path.Join(s.T().TempDir(), "cluster.yaml")

  // // export a template

  // exportStdout, exportStderr, err := s.runOmnictlCommand(ctx, "cluster template export -c talos-default -o "+templatePath+" -f", nil)
  // s.Require().NoError(err, "failed to export cluster template. stdout: %s, stderr: %s", exportStdout, exportStderr)

  // // collect resources before syncing the template back to the cluster

  // clusterBefore, _, err := s.runOmnictlCommand(ctx, "get cluster talos-default -ojson", nil)
  // s.Require().NoError(err, "failed to get cluster before export. stdout: %s", clusterBefore)

  // configPatchesBefore, _, err := s.runOmnictlCommand(ctx, "get configpatch -l omni.sidero.dev/cluster=talos-default -ojson", nil)
  // s.Require().NoError(err, "failed to get config patches before export. stdout: %s", configPatchesBefore)

  // // sync the template back to the cluster

  // syncStdout, syncStderr, err := s.runOmnictlCommand(ctx, "cluster template sync -f "+templatePath, nil)
  // s.Require().NoError(err, "failed to sync cluster. stdout: %s, stderr: %s", syncStdout, syncStderr)

  // // assert that only the cluster and a single config patch are updated

  // syncOutputLines := strings.Split(strings.TrimSpace(syncStdout), "\n")
  // if s.Len(syncOutputLines, 2, "sync output is not equal to expected") {
  // 	s.Equal("* updating Clusters.omni.sidero.dev(talos-default)", syncOutputLines[0], "sync output line 0 is not equal to expected")

  // 	// Assert the line to follow the format:
  // 	// * updating ConfigPatches.omni.sidero.dev(400-cm-f7f5fb9c-2aa3-42b7-b414-f2998ee153e9)
  // 	prefix := "* updating ConfigPatches.omni.sidero.dev(400-cm-"
  // 	suffix := ")"
  // 	hasPrefix := s.True(strings.HasPrefix(syncOutputLines[1], prefix), "sync output line 1 (%q) does not start with the expected prefix %q", syncOutputLines[1], prefix)
  // 	hasSuffix := s.True(strings.HasSuffix(syncOutputLines[1], suffix), "sync output line 1 (%q) does not end with the expected suffix %q", syncOutputLines[1], suffix)

  // 	if hasPrefix && hasSuffix {
  // 		machineID := strings.TrimPrefix(syncOutputLines[1], prefix)
  // 		machineID = strings.TrimSuffix(machineID, suffix)

  // 		_, err = uuid.Parse(machineID)
  // 		s.NoError(err, "machine id in sync output line 1 (%q) is not a valid UUID", syncOutputLines[1])
  // 	}
  // }

  // // assert the resource manifests are semantically equal before and after export

  // clusterAfter, _, err := s.runOmnictlCommand(ctx, "get cluster talos-default -ojson", nil)
  // s.Require().NoError(err, "failed to get cluster after export. stdout: %s", clusterAfter)

  // configPatchesAfter, _, err := s.runOmnictlCommand(ctx, "get configpatch -l omni.sidero.dev/cluster=talos-default -ojson", nil)
  // s.Require().NoError(err, "failed to get config patches after export. stdout: %s", configPatchesAfter)

  // diff, err := jsondiff.CompareJSON([]byte(clusterBefore), []byte(clusterAfter), jsondiff.Ignores("/metadata/updated", "/metadata/version", "/metadata/annotations"))
  // s.Require().NoError(err, "failed to compare cluster before and after export. diff: %s", diff)

  // s.Empty(diff.String(), "cluster before and after export are not equal")

  // configPatchListBefore := splitJSONObjects(s.T(), configPatchesBefore)
  // configPatchListAfter := splitJSONObjects(s.T(), configPatchesAfter)

  // s.Require().Equal(len(configPatchListBefore), len(configPatchListAfter), "config patch list before and after export are not equal")

  // var patchDataBefore, patchDataAfter string

  // for i, configPatchBefore := range configPatchListBefore {
  // 	configPatchAfter := configPatchListAfter[i]

  // 	diff, err = jsondiff.CompareJSON([]byte(configPatchBefore), []byte(configPatchAfter), jsondiff.Ignores("/metadata/updated", "/metadata/version", "/metadata/annotations", "/spec/data"))
  // 	s.Require().NoError(err, "failed to compare config patches before and after export. diff: %s", diff)

  // 	s.Empty(diff.String(), "config patches before and after export are not equal")

  // 	patchDataBefore, err = jsonparser.GetString([]byte(configPatchBefore), "spec", "data")
  // 	s.Require().NoError(err, "failed to get spec.data from config patches before export")

  // 	patchDataAfter, err = jsonparser.GetString([]byte(configPatchAfter), "spec", "data")
  // 	s.Require().NoError(err, "failed to get spec.data from config patches after export")

  // 	s.YAMLEq(patchDataBefore, patchDataAfter, "spec.data from config patches before and after export are not equal")
  // }
})
