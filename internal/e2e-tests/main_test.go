// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/buger/jsonparser"
	"github.com/google/uuid"
	"github.com/playwright-community/playwright-go"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/wI2L/jsondiff"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
)

const (
	omnictlPath    = "/usr/local/bin/omnictl"
	omniconfigPath = "/tmp/omniconfig"
	auditLogPath   = "/tmp/audit-log.jsonlog"
)

var True = true

type E2ESuite struct {
	suite.Suite

	pw       *playwright.Playwright
	chromium playwright.Browser

	baseURL  string
	videoDir string
	username string
	password string
}

func (s *E2ESuite) SetupTest() {
	var err error

	s.pw, err = playwright.Run()
	s.Require().NoError(err, "could not start playwright")

	s.chromium, err = s.pw.Chromium.Launch()
	s.Require().NoError(err, "could not launch browser Chromium")

	s.baseURL = os.Getenv("BASE_URL")
	s.Require().NotEmpty(s.baseURL, "BASE_URL is not set")
	s.T().Logf("BASE_URL is set to: %s", s.baseURL)

	s.videoDir = os.Getenv("VIDEO_DIR")
	if s.videoDir == "" {
		s.T().Logf("VIDEO_DIR env is not set, videos will not be saved")
	} else {
		s.T().Logf("videos will be saved to VIDEO_DIR: %s", s.videoDir)
	}

	s.username = os.Getenv("AUTH_USERNAME")
	s.Require().NotEmpty(s.username, "username is not set")

	s.password = os.Getenv("AUTH_PASSWORD")
	s.Require().NotEmpty(s.password, "password is not set")
}

type loginFlow func(*testing.T, playwright.Page)

func loginUser(t *testing.T, page playwright.Page) {
	logIn := page.Locator("button#login")

	require.NoError(t, logIn.WaitFor())

	require.NoError(t, logIn.Click())
}

func loginCLI(t *testing.T, page playwright.Page) {
	logIn := page.Locator("button#confirm")

	require.NoError(t, logIn.WaitFor())

	require.NoError(t, logIn.Click())

	require.NoError(t, page.Locator("#confirmed").WaitFor())
}

func (s *E2ESuite) withPage(url string, loginFlow loginFlow, f func(page playwright.Page)) {
	var recordVideo *playwright.RecordVideo

	if s.videoDir != "" {
		recordVideo = &playwright.RecordVideo{
			Dir: s.videoDir,
			Size: &playwright.Size{
				Width:  1280,
				Height: 720,
			},
		}
	}

	browserCtx, err := s.chromium.NewContext(playwright.BrowserNewContextOptions{
		AcceptDownloads:   &True,
		IgnoreHttpsErrors: &True,
		RecordVideo:       recordVideo,
	})
	s.Require().NoError(err)

	defer func() { s.Require().NoError(browserCtx.Close()) }()

	page, err := browserCtx.NewPage()
	s.Require().NoError(err)

	page.On("request", func(request playwright.Request) {
		s.T().Logf(">> %s %s", request.Method(), request.URL())
	})
	page.On("response", func(response playwright.Response) {
		s.T().Logf("<< %v %s", response.Status(), response.URL())
	})
	page.On("console", func(msg playwright.ConsoleMessage) {
		s.T().Logf("-- %s", msg)
	})

	_, err = page.Goto(url)
	s.Require().NoError(err)

	element := page.Locator("input#username").First()

	s.Require().NoError(element.Click())
	s.Require().NoError(element.Fill(s.username))

	element = page.Locator("input#password").First()

	s.Require().NoError(element.Click())
	s.Require().NoError(element.Fill(s.password))

	s.click(page, `button[type="submit"]:has-text("Continue"):visible`)

	loginFlow(s.T(), page)

	f(page)
}

func (s *E2ESuite) runOmnictlCommand(ctx context.Context, args string, stderrHandler func(line string)) (string, string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, omnictlPath,
		append([]string{"--omniconfig", omniconfigPath, "--insecure-skip-tls-verify"}, strings.Fields(args)...)...,
	)

	stdout := strings.Builder{}
	stderr := strings.Builder{}

	reader, writer := io.Pipe()

	cmd.Stdout = &stdout
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("failed to start command: %w", err)
	}

	eg := errgroup.Group{}
	lineCh := make(chan string)

	eg.Go(func() error {
		scanner := bufio.NewScanner(io.TeeReader(reader, &stderr))
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return nil
			case lineCh <- scanner.Text():
			}
		}

		return scanner.Err()
	})

	eg.Go(func() error {
		defer cancel()

		return errors.Join(cmd.Wait(), writer.Close(), ctx.Err())
	})

	for {
		select {
		case <-ctx.Done():
			return stdout.String(), stderr.String(), eg.Wait()
		case line := <-lineCh:
			if stderrHandler != nil {
				stderrHandler(line)
			}
		}
	}
}

func (s *E2ESuite) prepareOmnictl(ctx context.Context) {
	s.T().Logf("preparing omnictl")

	// download omnictl and omniconfig
	s.withPage(s.baseURL, loginUser, func(page playwright.Page) {
		downloadOmnictlButton := page.Locator(`span:has-text("Download omnictl"):visible`)

		s.Require().NoError(downloadOmnictlButton.Click())

		downloadButton := page.GetByText("Download", playwright.PageGetByTextOptions{
			Exact: playwright.Bool(true),
		})

		download, err := page.ExpectDownload(func() error {
			return downloadButton.Click()
		})
		s.Require().NoError(err, "failed to download omnictl")

		s.Require().NoError(download.SaveAs(omnictlPath), "failed to save omnictl")

		s.Require().NoError(os.Chmod(omnictlPath, 0o755), "failed to chmod omnictl")

		version, _, err := s.runOmnictlCommand(ctx, "--version", nil)
		s.Require().NoError(err, "failed to get omnictl version")

		s.T().Logf("omnictl version: %s", version)

		omniconfigDownload := page.Locator(`span:has-text("Download omniconfig"):visible`)

		download, err = page.ExpectDownload(func() error {
			return omniconfigDownload.Click()
		})
		s.Require().NoError(err, "failed to download omniconfig")

		s.Require().NoError(download.SaveAs(omniconfigPath), "failed to save omniconfig")
	})

	// go through the CLI auth flow
	rxStrict := xurls.Strict()
	loggedIn := false
	backendVersion, _, err := s.runOmnictlCommand(ctx, "get sysversion -ojsonpath={.spec.backendversion}", func(line string) {
		if loggedIn {
			return
		}

		authURL := rxStrict.FindString(line)
		if authURL == "" {
			return
		}

		s.T().Logf("Captured CLI Auth URL from stderr: %s", authURL)

		// We do not need to do anything in the callback function, withPage goes through the auth flow
		// for the auth URL by clicking the "Grant Access" button.
		s.withPage(authURL, loginCLI, func(page playwright.Page) {})
	})
	s.Require().NoError(err, "failed to prepare omnictl")

	s.T().Logf("backend version: %s", backendVersion)
}

func (s *E2ESuite) TestTitle() {
	expected := "Omni - default"

	s.withPage(s.baseURL, loginUser, func(page playwright.Page) {
		err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(func() error {
			title, err := page.Title()
			s.Require().NoError(err)

			if title != expected {
				return retry.ExpectedErrorf("expected title to be %q, got %q", expected, title)
			}

			return nil
		})

		s.NoError(err)
	})
}

func (s *E2ESuite) TestAuditLog() {
	s.T().Logf("getting audit log")

	s.withPage(s.baseURL, loginUser, func(page playwright.Page) {
		downloadAuditLog := page.Locator(`span:has-text("Get audit log"):visible`)

		s.Require().NoError(downloadAuditLog.Click())

		download, err := page.ExpectDownload(func() error { return downloadAuditLog.Click() })
		s.Require().NoError(err, "failed to download audit log")

		s.Require().NoError(download.SaveAs(auditLogPath), "failed to save audit log")
	})

	contents, err := os.ReadFile(auditLogPath)
	s.Require().NoError(err, "failed to read audit log")
	s.Require().Contains(string(contents), `"resource_type":"Identities.omni.sidero.dev"`)
}

func (s *E2ESuite) TestClickViewAll() {
	s.withPage(s.baseURL, loginUser, func(page playwright.Page) {
		viewAllButton := page.Locator("section").
			Filter(playwright.LocatorFilterOptions{HasText: "Recent Clusters"}).
			GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "View All"})

		err := viewAllButton.WaitFor()

		s.Require().NoError(err, "could not get entries")

		s.Require().NoError(viewAllButton.Click(), "error clicking button")

		expectedURL, err := url.JoinPath(s.baseURL, "/omni/clusters")
		s.Require().NoError(err)

		s.Equal(expectedURL, page.MainFrame().URL())
	})
}

//nolint:gocognit
func (s *E2ESuite) TestCreateCluster() {
	s.assertClusterCreation()
	s.assertTemplateExportAndSync()
}

func (s *E2ESuite) expandCluster(page playwright.Page, name string) {
	id := fmt.Sprintf("#%s-cluster-box", name)

	err := page.Locator(id).WaitFor(playwright.LocatorWaitForOptions{
		Timeout: nil,
	})
	s.Require().NoError(err)

	s.click(page, id)
}

func (s *E2ESuite) assertClusterCreation() {
	s.withPage(s.baseURL, loginUser, func(page playwright.Page) {
		navigateToClusters := func() {
			err := page.Locator(`#sidebar-menu-clusters`).WaitFor()

			s.Require().NoError(err)

			clustersURL, err := url.JoinPath(s.baseURL, "/omni/clusters")
			s.Require().NoError(err)

			s.navigate(page, `#sidebar-menu-clusters`, clustersURL)
		}

		navigateToClusters()

		// create cluster
		clusterCreateURL, err := url.JoinPath(s.baseURL, "/omni/cluster/create")
		s.Require().NoError(err)

		s.navigate(page, `button[type="button"]:has-text("Create Cluster")`, clusterCreateURL)

		s.click(page, `div.machine-set-label#CP >> nth=0`)

		s.click(page, `div.machine-set-label#W0 >> nth=1`)

		s.click(page, "button#CP")

		editor := page.Locator(`div.monaco-editor`).First()

		s.Require().NoError(editor.Click())

		selectAllKeyCombination := "Control+a"
		if runtime.GOOS == "darwin" {
			selectAllKeyCombination = "Meta+a"
		}

		s.Require().NoError(editor.Press(selectAllKeyCombination))
		s.Require().NoError(editor.Press("Delete"))
		s.Require().NoError(editor.PressSequentially(`machine:
 network:
   hostname: deadbeef`))

		s.click(page, `button:has-text("Save")`)

		err = page.Locator("button#extensions-CP").WaitFor()

		s.Require().NoError(err)

		s.click(page, `button#extensions-CP`)

		err = page.Locator(`span:has-text("usb-modem-drivers")`).WaitFor()

		s.Require().NoError(err)

		s.click(page, `span:has-text("usb-modem-drivers")`)

		s.click(page, `button:has-text("Save")`)

		s.click(page, `button:has-text("Create Cluster")`)

		err = page.GetByRole("heading", playwright.PageGetByRoleOptions{Name: "talos-default"}).WaitFor()
		s.Require().NoError(err)

		navigateToClusters()

		err = page.Locator("#talos-default-cluster-box").WaitFor()
		s.Require().NoError(err)

		clusterURL, err := url.JoinPath(s.baseURL, "/cluster/talos-default")
		s.Require().NoError(err)

		clusterOverviewURL, err := url.JoinPath(clusterURL, "/overview")
		s.Require().NoError(err)

		s.navigate(page, `div.clusters-grid a`, clusterOverviewURL)

		scaleURL, err := url.JoinPath(clusterURL, "/scale")
		s.Require().NoError(err)

		s.navigate(page, `button[type="button"]:has-text("Cluster Scaling")`, scaleURL)

		s.click(page, `div.machine-set-label#W0 >> nth=0`)

		s.click(page, `button:has-text("Update")`)

		// Wait for the scaling to navigate successfully to the cluster overview page to avoid a race condition where
		// the test navigates to the Clusters page too early, then the scaling succeeds and goes to the Cluster Overview page
		err = page.Locator(`text=Updated Cluster Configuration`).WaitFor()
		s.Require().NoError(err)

		openPage := func() {
			navigateToClusters()
		}

		openPage()

		s.NoError(retry.Constant(15*time.Minute, retry.WithUnits(5*time.Second)).Retry(func() error {
			element := page.Locator(`div.clusters-grid`)

			count, err := element.Count()
			if err != nil {
				openPage()

				return retry.ExpectedErrorf("failed to get clusters-grid element %s", err)
			}

			if count < 1 {
				return retry.ExpectedErrorf("the cluster is not created yet")
			}

			runningMachineSet := page.Locator(`div[id="machine-set-phase-name"]:has-text("Running")`)
			runningMachineSetCount, err := runningMachineSet.Count()
			if err != nil {
				openPage()

				return retry.ExpectedErrorf("failed to get running machine sets count %s", err)
			}

			runningMachines := page.Locator(`div[id="cluster-machine-stage-name"]:has-text("Running")`)
			runningMachinesCount, err := runningMachines.Count()
			if err != nil {
				openPage()

				return retry.ExpectedErrorf("failed to get running machines count %s", err)
			}

			machineCount := page.Locator("#machine-count")
			runningAndTotal, err := machineCount.TextContent()
			if err != nil {
				openPage()

				return retry.ExpectedErrorf("failed to get machines total %s", err)
			}

			_, machinesTotal, found := strings.Cut(runningAndTotal, "/")
			if !found {
				return fmt.Errorf("failed to parse machines count")
			}

			hostname := page.Locator(`text=deadbeef`)
			hostnameCount, err := hostname.Count()
			if err != nil {
				return retry.ExpectedErrorf("failed to get hostname %s", err)
			}

			s.T().Logf("machine sets running: %d/2, machines running: %d/%s, hostname set: %t", runningMachineSetCount, runningMachinesCount, machinesTotal, hostnameCount > 0)
			if hostnameCount == 0 {
				return retry.ExpectedErrorf("no machines with hostname deadbeef found, patch may not be applied")
			}

			if machinesTotal != "3" {
				return retry.ExpectedErrorf("total machines expected to be 3, got %s", machinesTotal)
			}

			if runningMachineSetCount == 2 && runningMachinesCount == 3 {
				return nil
			}

			return retry.ExpectedError(fmt.Errorf("cluster is not healthy"))
		}))

		s.click(page, "text=deadbeef")

		s.click(page, "text=Extensions")

		err = page.Locator(`text=siderolabs/usb-modem-drivers`).WaitFor()
		s.Require().NoError(err)

		element := page.Locator("text=siderolabs/usb-modem-drivers")
		count, err := element.Count()
		s.Require().NoError(err)

		s.Require().Equal(1, count)
	})
}

func (s *E2ESuite) assertTemplateExportAndSync() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 60*time.Second)
	s.T().Cleanup(cancel)

	s.prepareOmnictl(ctx)

	templatePath := path.Join(s.T().TempDir(), "cluster.yaml")

	// export a template

	exportStdout, exportStderr, err := s.runOmnictlCommand(ctx, "cluster template export -c talos-default -o "+templatePath+" -f", nil)
	s.Require().NoError(err, "failed to export cluster template. stdout: %s, stderr: %s", exportStdout, exportStderr)

	// collect resources before syncing the template back to the cluster

	clusterBefore, _, err := s.runOmnictlCommand(ctx, "get cluster talos-default -ojson", nil)
	s.Require().NoError(err, "failed to get cluster before export. stdout: %s", clusterBefore)

	configPatchesBefore, _, err := s.runOmnictlCommand(ctx, "get configpatch -l omni.sidero.dev/cluster=talos-default -ojson", nil)
	s.Require().NoError(err, "failed to get config patches before export. stdout: %s", configPatchesBefore)

	// sync the template back to the cluster

	syncStdout, syncStderr, err := s.runOmnictlCommand(ctx, "cluster template sync -f "+templatePath, nil)
	s.Require().NoError(err, "failed to sync cluster. stdout: %s, stderr: %s", syncStdout, syncStderr)

	// assert that only the cluster and a single config patch are updated

	syncOutputLines := strings.Split(strings.TrimSpace(syncStdout), "\n")
	if s.Len(syncOutputLines, 2, "sync output is not equal to expected") {
		s.Equal("* updating Clusters.omni.sidero.dev(talos-default)", syncOutputLines[0], "sync output line 0 is not equal to expected")

		// Assert the line to follow the format:
		// * updating ConfigPatches.omni.sidero.dev(400-cm-f7f5fb9c-2aa3-42b7-b414-f2998ee153e9)
		prefix := "* updating ConfigPatches.omni.sidero.dev(400-cm-"
		suffix := ")"
		hasPrefix := s.True(strings.HasPrefix(syncOutputLines[1], prefix), "sync output line 1 (%q) does not start with the expected prefix %q", syncOutputLines[1], prefix)
		hasSuffix := s.True(strings.HasSuffix(syncOutputLines[1], suffix), "sync output line 1 (%q) does not end with the expected suffix %q", syncOutputLines[1], suffix)

		if hasPrefix && hasSuffix {
			machineID := strings.TrimPrefix(syncOutputLines[1], prefix)
			machineID = strings.TrimSuffix(machineID, suffix)

			_, err = uuid.Parse(machineID)
			s.NoError(err, "machine id in sync output line 1 (%q) is not a valid UUID", syncOutputLines[1])
		}
	}

	// assert the resource manifests are semantically equal before and after export

	clusterAfter, _, err := s.runOmnictlCommand(ctx, "get cluster talos-default -ojson", nil)
	s.Require().NoError(err, "failed to get cluster after export. stdout: %s", clusterAfter)

	configPatchesAfter, _, err := s.runOmnictlCommand(ctx, "get configpatch -l omni.sidero.dev/cluster=talos-default -ojson", nil)
	s.Require().NoError(err, "failed to get config patches after export. stdout: %s", configPatchesAfter)

	diff, err := jsondiff.CompareJSON([]byte(clusterBefore), []byte(clusterAfter), jsondiff.Ignores("/metadata/updated", "/metadata/version", "/metadata/annotations"))
	s.Require().NoError(err, "failed to compare cluster before and after export. diff: %s", diff)

	s.Empty(diff.String(), "cluster before and after export are not equal")

	configPatchListBefore := splitJSONObjects(s.T(), configPatchesBefore)
	configPatchListAfter := splitJSONObjects(s.T(), configPatchesAfter)

	s.Require().Equal(len(configPatchListBefore), len(configPatchListAfter), "config patch list before and after export are not equal")

	var patchDataBefore, patchDataAfter string

	for i, configPatchBefore := range configPatchListBefore {
		configPatchAfter := configPatchListAfter[i]

		diff, err = jsondiff.CompareJSON([]byte(configPatchBefore), []byte(configPatchAfter), jsondiff.Ignores("/metadata/updated", "/metadata/version", "/metadata/annotations", "/spec/data"))
		s.Require().NoError(err, "failed to compare config patches before and after export. diff: %s", diff)

		s.Empty(diff.String(), "config patches before and after export are not equal")

		patchDataBefore, err = jsonparser.GetString([]byte(configPatchBefore), "spec", "data")
		s.Require().NoError(err, "failed to get spec.data from config patches before export")

		patchDataAfter, err = jsonparser.GetString([]byte(configPatchAfter), "spec", "data")
		s.Require().NoError(err, "failed to get spec.data from config patches after export")

		s.YAMLEq(patchDataBefore, patchDataAfter, "spec.data from config patches before and after export are not equal")
	}
}

// splitJSONObjects splits a stream of JSON objects into individual JSON objects.
func splitJSONObjects(t *testing.T, input string) []string {
	decoder := json.NewDecoder(strings.NewReader(input))

	var result []string

	for {
		var data map[string]any

		err := decoder.Decode(&data)
		if err == io.EOF {
			// all done
			break
		}

		marshaled, err := json.Marshal(data)
		require.NoError(t, err)

		result = append(result, string(marshaled))
	}

	return result
}

func (s *E2ESuite) TestOpenMachine() {
	clustersURL, err := url.JoinPath(s.baseURL, "/omni/clusters")
	s.Require().NoError(err)

	s.withPage(clustersURL, loginUser, func(page playwright.Page) {
		s.expandCluster(page, "talos-default")

		node := page.Locator("#talos-default-control-planes > div:last-child")

		s.Require().NoError(node.WaitFor())
		s.Require().NoError(node.Click())

		s.Require().NoError(page.Locator("#etcd").WaitFor())

		_, err = page.Goto(clustersURL)
		s.Require().NoError(err)

		node = page.Locator("#talos-default-workers > div:last-child")

		s.Require().NoError(node.WaitFor())
		s.Require().NoError(node.Click())

		s.Require().NoError(page.Locator("#machined").WaitFor())

		element := page.Locator("text=etcd")
		count, err := element.Count()
		s.Require().NoError(err)

		s.Require().Equal(0, count)
	})
}

func (s *E2ESuite) click(page playwright.Page, locator string) {
	s.Require().NoErrorf(page.Locator(locator).First().Click(), "error clicking to the first element matching locator: %q", locator)
}

func (s *E2ESuite) navigate(page playwright.Page, locator, expectedURL string) {
	s.click(page, locator)

	s.Require().NoError(page.WaitForURL(expectedURL), "failed to wait for URL %q after clicking on the first element with locator %q (current URL: %q)", expectedURL, locator, page.URL())
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, &E2ESuite{})
}
