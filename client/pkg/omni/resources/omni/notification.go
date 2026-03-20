// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewNotification creates new Notification resource.
func NewNotification(id resource.ID) *Notification {
	return typed.NewResource[NotificationSpec, NotificationExtension](
		resource.NewMetadata(resources.EphemeralNamespace, NotificationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.NotificationSpec{}),
	)
}

const (
	// NotificationType is the type of the Notification resource.
	// tsgen:NotificationType
	NotificationType = resource.Type("Notifications.omni.sidero.dev")

	// NotificationMachineRegistrationLimitID is the ID for the machine registration limit notification.
	// tsgen:NotificationMachineRegistrationLimitID
	NotificationMachineRegistrationLimitID = "machine-registration-limit"

	// NotificationNonImageFactoryMachinesID is the ID for the non-ImageFactory machines deprecation notification.
	// tsgen:NotificationNonImageFactoryMachinesID
	NotificationNonImageFactoryMachinesID = "non-image-factory-machines"
)

// Notification describes a generic notification emitted by a controller.
type Notification = typed.Resource[NotificationSpec, NotificationExtension]

// NotificationSpec wraps specs.NotificationSpec.
type NotificationSpec = protobuf.ResourceSpec[specs.NotificationSpec, *specs.NotificationSpec]

// NotificationExtension provides auxiliary methods for Notification resource.
type NotificationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NotificationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NotificationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
