// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package exposedservice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/exposedservice"
)

func TestIsExposedServiceEvent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct { //nolint:govet
		name          string
		obj           any
		oldObj        any
		expectChanged bool
	}{
		{
			name:          "add/remove unrelated service",
			expectChanged: false,
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name:          "add/remove service - missing port annotation",
			expectChanged: false,
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServiceLabelAnnotationKey: "foo",
						constants.ExposedServiceIconAnnotationKey:  "bar",
					},
				},
			},
		},
		{
			name:          "add/remove service - exposed",
			expectChanged: true,
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServicePortAnnotationKey: "8080",
					},
				},
			},
		},
		{
			name:          "update service - no change in exposed service annotations",
			expectChanged: false,
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServicePortAnnotationKey:  "8080",
						constants.ExposedServiceLabelAnnotationKey: "foo",
					},
				},
				Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
			},
			oldObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServicePortAnnotationKey:  "8080",
						constants.ExposedServiceLabelAnnotationKey: "foo",
					},
				},
				Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.2"},
			},
		},
		{
			name:          "update service - change in exposed service annotations",
			expectChanged: true,
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServicePortAnnotationKey:  "8080",
						constants.ExposedServiceLabelAnnotationKey: "foo",
					},
				},
			},
			oldObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ExposedServicePortAnnotationKey:  "8080",
						constants.ExposedServiceLabelAnnotationKey: "bar", // different label
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			changed := exposedservice.IsExposedServiceEvent(tt.obj, tt.oldObj, zaptest.NewLogger(t))

			assert.Equal(t, tt.expectChanged, changed)
		})
	}
}
