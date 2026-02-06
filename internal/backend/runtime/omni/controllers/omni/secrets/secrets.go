// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package secrets contains the Controller.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// Controller creates omni.ClusterSecrets for each inputed omni.Cluster.
//
// Controller generates and stores initial cluster-wide secrets.
type Controller struct {
	*qtransform.QController[*omni.Cluster, *omni.ClusterSecrets]
}

// NewSecretsController instantiates the secrets' controller.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func NewSecretsController(etcdBackupStoreFactory store.Factory) *Controller {
	ctrl := &Controller{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterSecrets]{
			Name: "SecretsController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(secrets *omni.ClusterSecrets) *omni.Cluster {
				return omni.NewCluster(secrets.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cluster *omni.Cluster, secrets *omni.ClusterSecrets) error {
				if len(secrets.TypedSpec().Value.GetData()) != 0 {
					// the cluster already has secrets, skipping if secret rotation is not required
					secretRotation, err := safe.ReaderGetByID[*omni.SecretRotation](ctx, r, cluster.Metadata().ID())
					if err != nil && !state.IsNotFoundError(err) {
						return err
					}

					if secretRotationVersion, _ := secrets.Metadata().Annotations().Get(omni.SecretRotationVersion); secretRotation != nil &&
						secretRotationVersion != secretRotation.Metadata().Version().String() {
						if err = ctrl.handleSecretRotation(secrets, secretRotation); err != nil {
							return err
						}
					}

					return nil
				}

				versionContract, err := config.ParseContractFromVersion("v" + cluster.TypedSpec().Value.TalosVersion)
				if err != nil {
					return err
				}

				cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
				if err != nil {
					if state.IsNotFoundError(err) { // need to wait for the control plane to be created, so we can decide if we need to get secrets from an etcd backup or not
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				// Here, we need to use uncached reader, since once Secrets is created, it will never be attempted again,
				// because of this, we cannot tolerate a stale read - later reconciliation will not put things back in order.
				// The precise sequence of events we want to avoid is:
				//
				// t0: ImportedClusterSecrets is created
				// t1: Control Plane MachineSet is created
				// t2: The controller wakes up due to MachineSet was mapped to Cluster:
				// - it can read the MachineSet and proceeds
				// - despite ImportedClusterSecrets was created before, it is not yet visible to the controller due to the controller runtime cache
				// - the controller proceeds to create a new bundle
				// t3: ImportedClusterSecrets notification wakes up the controller, but the bundle is already there, the controller does not do anything

				uncachedReader, ok := r.(controller.UncachedReader)
				if !ok {
					return fmt.Errorf("reader does not support uncached reads")
				}

				icsRes, err := uncachedReader.GetUncached(ctx, omni.NewImportedClusterSecrets(cluster.Metadata().ID()).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if icsRes != nil {
					ics, icsOk := icsRes.(*omni.ImportedClusterSecrets)
					if !icsOk {
						return fmt.Errorf("unexpected resource type: %T", icsRes)
					}

					var bundle *talossecrets.Bundle

					bundle, err = omni.FromImportedSecretsToSecretsBundle(ics)
					if err != nil {
						return fmt.Errorf("failed to decode imported cluster secrets: %w", err)
					}

					var data []byte

					data, err = json.Marshal(bundle)
					if err != nil {
						return fmt.Errorf("error marshaling secrets: %w", err)
					}

					secrets.TypedSpec().Value.Imported = true
					secrets.TypedSpec().Value.Data = data

					return nil
				}

				bundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), versionContract)
				if err != nil {
					return fmt.Errorf("error generating secrets: %w", err)
				}

				bootstrapSpec := cpMachineSet.TypedSpec().Value.GetBootstrapSpec()
				if bootstrapSpec != nil {
					var backupData etcdbackup.BackupData

					backupData, err = ctrl.getBackupDataFromBootstrapSpec(ctx, r, etcdBackupStoreFactory, bootstrapSpec)
					if err != nil {
						return err
					}

					bundle.Secrets.AESCBCEncryptionSecret = backupData.AESCBCEncryptionSecret
					bundle.Secrets.SecretboxEncryptionSecret = backupData.SecretboxEncryptionSecret
				}

				data, err := json.Marshal(bundle)
				if err != nil {
					return fmt.Errorf("error marshaling secrets: %w", err)
				}

				secrets.TypedSpec().Value.Imported = false
				secrets.TypedSpec().Value.Data = data

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster) error {
				clusterMachines, err := r.List(
					ctx,
					resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, "", resource.VersionUndefined),
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				if len(clusterMachines.Items) != 0 {
					// cluster still has machines, skipping
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster %q still has machines", cluster.Metadata().ID())
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			mappers.MapByClusterLabelOnlyControlplane[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterUUID](
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.BackupData](
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.ImportedClusterSecrets](
			qtransform.MapperSameID[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.SecretRotation](
			qtransform.MapperSameID[*omni.Cluster](),
		),
	)

	return ctrl
}

func (s *Controller) getBackupDataFromBootstrapSpec(
	ctx context.Context,
	r controller.Reader,
	etcdBackupStoreFactory store.Factory,
	spec *specs.MachineSetSpec_BootstrapSpec,
) (etcdbackup.BackupData, error) {
	backupStore, err := etcdBackupStoreFactory.GetStore()
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to get backup store: %w", err)
	}

	clusterUUIDList, err := safe.ReaderListAll[*omni.ClusterUUID](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterUUID, spec.GetClusterUuid())))
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to list cluster UUIDs for cluster %q: %w", spec.GetClusterUuid(), err)
	}

	if clusterUUIDList.Len() == 0 {
		return etcdbackup.BackupData{}, fmt.Errorf("cluster UUID %q not found", spec.GetClusterUuid())
	} else if clusterUUIDList.Len() > 1 {
		return etcdbackup.BackupData{}, fmt.Errorf("multiple cluster UUIDs found for cluster %q", spec.GetClusterUuid())
	}

	clusterUUID := clusterUUIDList.Get(0)
	clusterID := clusterUUID.Metadata().ID()

	backupData, err := safe.ReaderGetByID[*omni.BackupData](ctx, r, clusterID)
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to get backup data for cluster %q: %w", clusterID, err)
	}

	downloadedBackupData, readCloser, err := backupStore.Download(ctx, backupData.TypedSpec().Value.GetEncryptionKey(), spec.GetClusterUuid(), spec.GetSnapshot())
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to download backup data for cluster %q: %w", clusterID, err)
	}

	defer readCloser.Close() //nolint:errcheck

	return downloadedBackupData, nil
}

func (s *Controller) handleSecretRotation(
	secrets *omni.ClusterSecrets,
	secretRotation *omni.SecretRotation,
) error {
	bundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return fmt.Errorf("failed to decode secrets: %w", err)
	}

	switch secretRotation.TypedSpec().Value.Phase {
	case specs.SecretRotationSpec_OK:
		secrets.TypedSpec().Value.ExtraCerts = nil
	case specs.SecretRotationSpec_PRE_ROTATE:
		s.handlePreRotate(secrets, secretRotation)
	case specs.SecretRotationSpec_ROTATE:
		s.handleRotate(secrets, secretRotation, bundle)
	case specs.SecretRotationSpec_POST_ROTATE:
		s.handlePostRotate(secrets, secretRotation)
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("error marshaling secrets: %w", err)
	}

	secrets.TypedSpec().Value.Data = data
	secrets.Metadata().Annotations().Set(omni.SecretRotationVersion, secretRotation.Metadata().Version().String())

	return nil
}

func (s *Controller) handlePreRotate(secrets *omni.ClusterSecrets, secretRotation *omni.SecretRotation) {
	switch secretRotation.TypedSpec().Value.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		secrets.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
			Os: &specs.ClusterSecretsSpec_Certs_CA{
				Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt,
				Key: secretRotation.TypedSpec().Value.ExtraCerts.Os.Key,
			},
		}
	case specs.SecretRotationSpec_KUBERNETES_CA:
		secrets.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
			K8S: &specs.ClusterSecretsSpec_Certs_CA{
				Crt: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Crt,
				Key: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Key,
			},
		}
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}
}

func (s *Controller) handleRotate(secrets *omni.ClusterSecrets, secretRotation *omni.SecretRotation, bundle *talossecrets.Bundle) {
	switch secretRotation.TypedSpec().Value.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		bundle.Certs.OS = &x509.PEMEncodedCertificateAndKey{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.Os.Key,
		}
		secrets.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
			Os: &specs.ClusterSecretsSpec_Certs_CA{
				Crt: secretRotation.TypedSpec().Value.Certs.Os.Crt,
				Key: secretRotation.TypedSpec().Value.Certs.Os.Key,
			},
		}
	case specs.SecretRotationSpec_KUBERNETES_CA:
		bundle.Certs.K8s = &x509.PEMEncodedCertificateAndKey{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Key,
		}
		secrets.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
			K8S: &specs.ClusterSecretsSpec_Certs_CA{
				Crt: secretRotation.TypedSpec().Value.Certs.K8S.Crt,
				Key: secretRotation.TypedSpec().Value.Certs.K8S.Key,
			},
		}
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}
}

func (s *Controller) handlePostRotate(secrets *omni.ClusterSecrets, secretRotation *omni.SecretRotation) {
	switch secretRotation.TypedSpec().Value.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		secrets.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, strconv.Itoa(int(time.Now().Unix())))
	case specs.SecretRotationSpec_KUBERNETES_CA:
		secrets.Metadata().Annotations().Set(omni.RotateKubernetesCATimestamp, strconv.Itoa(int(time.Now().Unix())))
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}
}
