// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talosupgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testHealthCheckJobManifest = `apiVersion: batch/v1
kind: Job
metadata:
  name: user-chosen-name
spec:
  backoffLimit: 0
  template:
    spec:
      serviceAccountName: my-sa
      restartPolicy: Never
      containers:
        - name: check
          image: alpine
          command: ["true"]
`

func TestComposeHealthCheckJob(t *testing.T) {
	t.Parallel()

	job, err := composeHealthCheckJob("omni-healthcheck-x", testHealthCheckJobManifest)
	require.NoError(t, err)

	// Omni owns the name, so the manifest's name is overridden
	assert.Equal(t, "omni-healthcheck-x", job.Name)
	// namespace defaults when the manifest doesn't set one
	assert.Equal(t, defaultHealthCheckNamespace, job.Namespace)
	assert.Equal(t, healthCheckRunnerName, job.Labels["app.kubernetes.io/name"])
	assert.NotEmpty(t, job.Annotations[healthCheckConfigHashAnnotation])

	// the user's spec is preserved
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(0), *job.Spec.BackoffLimit)
	assert.Equal(t, "my-sa", job.Spec.Template.Spec.ServiceAccountName)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "alpine", job.Spec.Template.Spec.Containers[0].Image)
	// Omni defaults the termination message policy so the container's failure output can be read from the pod status
	assert.Equal(t, corev1.TerminationMessageFallbackToLogsOnError, job.Spec.Template.Spec.Containers[0].TerminationMessagePolicy)
}

func TestComposeHealthCheckJob_KeepsTerminationMessagePolicy(t *testing.T) {
	t.Parallel()

	manifest := "apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      containers:\n        - name: check\n          image: alpine\n          terminationMessagePolicy: File\n"

	job, err := composeHealthCheckJob("omni-healthcheck-x", manifest)
	require.NoError(t, err)

	// the user's explicit choice is preserved
	assert.Equal(t, corev1.TerminationMessageReadFile, job.Spec.Template.Spec.Containers[0].TerminationMessagePolicy)
}

func TestFailedContainerMessage(t *testing.T) {
	t.Parallel()

	pod := func(name, message string, exitCode int32, finishedAt int64) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode:   exitCode,
					Message:    message,
					FinishedAt: metav1.Unix(finishedAt, 0),
				}},
			}}},
		}
	}

	// the most recently finished failed container wins
	assert.Equal(t, "second failure", failedContainerMessage([]corev1.Pod{
		pod("a", "first failure", 1, 100),
		pod("b", "second failure", 1, 200),
	}))

	// successful containers are ignored
	assert.Empty(t, failedContainerMessage([]corev1.Pod{pod("a", "ignored", 0, 100)}))

	// no terminated containers -> empty
	assert.Empty(t, failedContainerMessage([]corev1.Pod{{}}))
}

func TestComposeHealthCheckJob_NamespaceFromManifest(t *testing.T) {
	t.Parallel()

	manifest := "apiVersion: batch/v1\nkind: Job\nmetadata:\n  namespace: custom-ns\nspec:\n  template:\n    spec:\n      containers:\n        - name: check\n          image: alpine\n"

	job, err := composeHealthCheckJob("omni-healthcheck-x", manifest)
	require.NoError(t, err)

	assert.Equal(t, "custom-ns", job.Namespace)
}

func TestComposeHealthCheckJob_ConfigHashChangesWithManifest(t *testing.T) {
	t.Parallel()

	a, err := composeHealthCheckJob("omni-healthcheck-x", testHealthCheckJobManifest)
	require.NoError(t, err)

	b, err := composeHealthCheckJob("omni-healthcheck-x", testHealthCheckJobManifest+"# changed\n")
	require.NoError(t, err)

	assert.NotEqual(
		t,
		a.Annotations[healthCheckConfigHashAnnotation],
		b.Annotations[healthCheckConfigHashAnnotation],
	)
}

func TestJobConditions(t *testing.T) {
	t.Parallel()

	complete := &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}}}
	assert.True(t, jobComplete(complete))
	assert.Nil(t, jobFailedCondition(complete))

	failed := &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit"},
	}}}
	assert.False(t, jobComplete(failed))

	cond := jobFailedCondition(failed)
	require.NotNil(t, cond)
	assert.Equal(t, "BackoffLimitExceeded", cond.Reason)
	assert.Equal(t, "Job has reached the specified backoff limit", cond.Message)

	// a job with a false/absent condition is neither complete nor failed
	pending := &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionFalse},
	}}}
	assert.False(t, jobComplete(pending))
	assert.Nil(t, jobFailedCondition(pending))
}
