// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/grpc/gen"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/secretrotation"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/sequence"
)

// RotationStatusControllerName is the name of the SecretRotationStatusController.
const RotationStatusControllerName = "SecretRotationStatusController"

// RotationPaused represents the status indicating that the rotation process is currently paused.
const RotationPaused = "rotation paused"

// BackedUpRotatedSecretsLimit is the maximum number of backed-up rotated secrets to keep.
const BackedUpRotatedSecretsLimit = 5

// TalosRemoteGeneratorFactory is the factory for providing a client for accessing trustd.
type TalosRemoteGeneratorFactory struct{}

func (f *TalosRemoteGeneratorFactory) NewRemoteGenerator(token string, endpoints []string, acceptedCAs []*x509.PEMEncodedCertificate) (secretrotation.RemoteGenerator, error) {
	remoteGenerator, err := gen.NewRemoteGenerator(token, endpoints, acceptedCAs)
	if err != nil {
		return nil, err
	}

	return remoteGenerator, err
}

type KubernetesClientFactory struct{}

func (f *KubernetesClientFactory) NewClient(config *rest.Config) (secretrotation.KubernetesClient, error) {
	result, err := kubernetes.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return result, nil
}

// NewSecretRotationStatusController instantiates the secret rotation status controller.
func NewSecretRotationStatusController(
	remoteGenFactory secretrotation.RemoteGeneratorFactory,
	kubernetesClientFactory secretrotation.KubernetesClientFactory,
) *sequence.Controller[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus] {
	return sequence.NewController[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus](
		RotationStatusControllerName,
		&Rotator{RemoteGeneratorFactory: remoteGenFactory, KubernetesClientFactory: kubernetesClientFactory},
	)
}

type Rotator struct {
	RemoteGeneratorFactory  secretrotation.RemoteGeneratorFactory
	KubernetesClientFactory secretrotation.KubernetesClientFactory
}

// Stages return the stages of the secret rotation status controller.
func (s *Rotator) Stages() []sequence.Stage[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus] {
	return []sequence.Stage[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus]{
		sequence.NewStage(specs.SecretRotationSpec_OK.String(), s.createInitialStage(specs.SecretRotationSpec_OK)),
		sequence.NewStage(specs.SecretRotationSpec_PRE_ROTATE.String(), s.addRotationStage(specs.SecretRotationSpec_OK, specs.SecretRotationSpec_PRE_ROTATE)),
		sequence.NewStage(specs.SecretRotationSpec_ROTATE.String(), s.addRotationStage(specs.SecretRotationSpec_PRE_ROTATE, specs.SecretRotationSpec_ROTATE)),
		sequence.NewStage(specs.SecretRotationSpec_POST_ROTATE.String(), s.addRotationStage(specs.SecretRotationSpec_ROTATE, specs.SecretRotationSpec_POST_ROTATE)),
	}
}

func (s *Rotator) MapFunc(res *omni.ClusterSecrets) *omni.ClusterSecretsRotationStatus {
	return omni.NewClusterSecretsRotationStatus(res.Metadata().ID())
}

func (s *Rotator) UnmapFunc(res *omni.ClusterSecretsRotationStatus) *omni.ClusterSecrets {
	return omni.NewClusterSecrets(res.Metadata().ID())
}

func (s *Rotator) Options() []qtransform.ControllerOption {
	return []qtransform.ControllerOption{
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByClusterLabel[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.LoadBalancerConfig](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.MachinePendingUpdates](
			mappers.MapByClusterLabel[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterConfigVersion](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.RotateTalosCA](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.RotateKubernetesCA](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.SecretRotation](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.SecretRotationType,
			Kind: controller.OutputExclusive,
		}),
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.ClusterMachineSecretsType,
			Kind: controller.OutputExclusive,
		}),
	}
}

func (s *Rotator) FinalizerRemoval(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, input *omni.ClusterSecrets) error {
	cmSecrets, err := safe.ReaderListAll[*omni.ClusterMachineSecrets](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, input.Metadata().ID())))
	if err != nil {
		return err
	}

	for secret := range cmSecrets.All() {
		if err = s.destroyClusterMachineSecret(ctx, r, logger, secret.Metadata().ID()); err != nil {
			return err
		}
	}

	logger.Debug("started destroy operation for ClusterMachineSecrets", zap.Int("count", cmSecrets.Len()))

	if cmSecrets.Len() > 0 {
		return controller.NewRequeueInterval(time.Second)
	}

	destroyed, err := helpers.TeardownAndDestroy(ctx, r, omni.NewSecretRotation(input.Metadata().ID()).Metadata())
	if err != nil {
		return err
	}

	if !destroyed {
		logger.Debug("waiting for SecretRotation to be destroyed")

		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("resource SecretRotation isn't destroyed yet"))
	}

	return nil
}

//nolint:gocognit
func (s *Rotator) createInitialStage(currentPhase specs.SecretRotationSpec_Phase) func(
	ctx context.Context,
	logger *zap.Logger,
	sequenceContext sequence.Context[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus],
) error {
	return func(ctx context.Context, logger *zap.Logger, sequenceContext sequence.Context[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus]) error {
		r := sequenceContext.Runtime
		clusterSecrets := sequenceContext.Input
		rotationStatus := sequenceContext.Output

		rotationStatus.TypedSpec().Value.Component = specs.SecretRotationSpec_NONE
		rotationStatus.TypedSpec().Value.Phase = currentPhase
		rotationStatus.TypedSpec().Value.Status = ""
		rotationStatus.TypedSpec().Value.Step = ""
		rotationStatus.TypedSpec().Value.Error = ""

		cmStatusesMap, cmSecretsMap, err := s.getCMStatusesAndSecrets(ctx, r, clusterSecrets.Metadata().ID())
		if err != nil {
			return err
		}

		secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
		if err != nil {
			return fmt.Errorf("failed to unmarshal cluster secrets bundle: %w", err)
		}

		secretsToCreate := s.toCreate(cmStatusesMap, cmSecretsMap)
		for _, clusterMachineStatus := range secretsToCreate {
			if err = s.modifyClusterMachineSecret(ctx, r, logger, secretsBundle, clusterMachineStatus, nil); err != nil {
				return err
			}
		}

		secretsToDestroy := s.toDestroy(cmStatusesMap, cmSecretsMap)
		for _, clusterMachineStatus := range secretsToDestroy {
			if err = s.destroyClusterMachineSecret(ctx, r, logger, clusterMachineStatus.Metadata().ID()); err != nil {
				return err
			}
		}

		secretRotation, err := safe.WriterModifyWithResult[*omni.SecretRotation](ctx, r, omni.NewSecretRotation(clusterSecrets.Metadata().ID()),
			func(res *omni.SecretRotation) error {
				res.TypedSpec().Value.Status = specs.SecretRotationSpec_IDLE
				res.TypedSpec().Value.Phase = currentPhase
				res.TypedSpec().Value.Component = specs.SecretRotationSpec_NONE
				res.TypedSpec().Value.Certs = &specs.ClusterSecretsSpec_Certs{
					Os: &specs.ClusterSecretsSpec_Certs_CA{
						Crt: secretsBundle.Certs.OS.Crt,
						Key: secretsBundle.Certs.OS.Key,
					},
					K8S: &specs.ClusterSecretsSpec_Certs_CA{
						Crt: secretsBundle.Certs.K8s.Crt,
						Key: secretsBundle.Certs.K8s.Key,
					},
				}
				res.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{}

				return nil
			})
		if err != nil {
			return err
		}

		rotateTalosCA, err := safe.ReaderGetByID[*omni.RotateTalosCA](ctx, r, clusterSecrets.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		// This is a trigger condition to move to the next stage
		if version, _ := rotationStatus.Metadata().Annotations().Get(omni.RotateTalosCAVersion); rotateTalosCA != nil && version != rotateTalosCA.Metadata().Version().String() {
			logger.Info("starting rotation", zap.String("cluster", secretRotation.Metadata().ID()), zap.String("component", specs.SecretRotationSpec_TALOS_CA.String()))

			return s.startCARotation(rotationStatus, cmSecretsMap, specs.SecretRotationSpec_TALOS_CA, rotateTalosCA.Metadata().Version().String(),
				func() (ca *x509.CertificateAuthority, err error) {
					return talossecrets.NewTalosCA(talossecrets.NewFixedClock(time.Now()).Now())
				},
				func(rotatingComponent specs.SecretRotationSpec_Component, ca *x509.CertificateAuthority) error {
					return safe.WriterModify[*omni.SecretRotation](ctx, r, secretRotation,
						func(res *omni.SecretRotation) error {
							res.TypedSpec().Value.Status = specs.SecretRotationSpec_IN_PROGRESS
							res.TypedSpec().Value.Component = rotatingComponent
							res.TypedSpec().Value.ExtraCerts.Os = &specs.ClusterSecretsSpec_Certs_CA{
								Crt: ca.CrtPEM,
								Key: ca.KeyPEM,
							}

							return nil
						})
				})
		}

		rotateKubernetesCA, err := safe.ReaderGetByID[*omni.RotateKubernetesCA](ctx, r, clusterSecrets.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		// This is a trigger condition to move to the next stage
		if version, _ := rotationStatus.Metadata().Annotations().Get(omni.RotateKubernetesCAVersion); rotateKubernetesCA != nil && version != rotateKubernetesCA.Metadata().Version().String() {
			logger.Info("starting rotation", zap.String("cluster", secretRotation.Metadata().ID()), zap.String("component", specs.SecretRotationSpec_KUBERNETES_CA.String()))

			return s.startCARotation(rotationStatus, cmSecretsMap, specs.SecretRotationSpec_KUBERNETES_CA, rotateKubernetesCA.Metadata().Version().String(),
				func() (ca *x509.CertificateAuthority, err error) {
					clusterConfigVersion, err := safe.ReaderGetByID[*omni.ClusterConfigVersion](ctx, r, secretRotation.Metadata().ID())
					if err != nil {
						return nil, err
					}

					versionContract, err := config.ParseContractFromVersion(clusterConfigVersion.TypedSpec().Value.Version)
					if err != nil {
						return nil, err
					}

					return talossecrets.NewKubernetesCA(talossecrets.NewFixedClock(time.Now()).Now(), versionContract)
				},
				func(rotatingComponent specs.SecretRotationSpec_Component, ca *x509.CertificateAuthority) error {
					return safe.WriterModify[*omni.SecretRotation](ctx, r, secretRotation,
						func(res *omni.SecretRotation) error {
							res.TypedSpec().Value.Status = specs.SecretRotationSpec_IN_PROGRESS
							res.TypedSpec().Value.Component = rotatingComponent
							res.TypedSpec().Value.ExtraCerts.K8S = &specs.ClusterSecretsSpec_Certs_CA{
								Crt: ca.CrtPEM,
								Key: ca.KeyPEM,
							}

							return nil
						})
				})
		}

		return sequence.ErrWait // Continue processing the current stage
	}
}

//nolint:gocognit
func (s *Rotator) addRotationStage(previousPhase, currentPhase specs.SecretRotationSpec_Phase) func(
	ctx context.Context,
	logger *zap.Logger,
	sequenceContext sequence.Context[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus],
) error {
	return func(ctx context.Context, logger *zap.Logger, sequenceContext sequence.Context[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus]) error {
		r := sequenceContext.Runtime
		clusterSecrets := sequenceContext.Input
		rotationStatus := sequenceContext.Output
		rotationStatus.TypedSpec().Value.Phase = currentPhase

		secretRotation, err := s.getSecretRotation(ctx, r, clusterSecrets.Metadata().ID())
		if err != nil {
			return err
		}

		cmStatusesMap, cmSecretsMap, err := s.getCMStatusesAndSecrets(ctx, r, clusterSecrets.Metadata().ID())
		if err != nil {
			return err
		}

		if secretRotation.TypedSpec().Value.Status != specs.SecretRotationSpec_IN_PROGRESS {
			return fmt.Errorf("waiting for SecretRotation to be in progress, current status: %s", secretRotation.TypedSpec().Value.Status.String())
		}

		// Phase transition for SecretRotation is delayed until all machines are processed in the previous phase.
		// Here we check for the previous phase and wait for that change to be reflected on SecretRotation
		if secretRotation.TypedSpec().Value.Phase != previousPhase {
			return fmt.Errorf("waiting for phase transition for SecretRotation to be updated, current phase: %s, expected phase: %s",
				secretRotation.TypedSpec().Value.Phase.String(), previousPhase.String())
		}

		if err = s.handleClusterMachineScaling(ctx, r, logger, clusterSecrets, secretRotation, cmStatusesMap, cmSecretsMap, previousPhase, currentPhase); err != nil {
			return err
		}

		if err = s.handleClusterMachineSecretRotation(ctx, r, logger, rotationStatus, secretRotation, cmSecretsMap, cmStatusesMap, currentPhase); err != nil {
			return err
		}

		logger.Info("finished phase for CA rotation", zap.String("cluster", clusterSecrets.Metadata().ID()),
			zap.String("component", secretRotation.TypedSpec().Value.Component.String()), zap.String("phase", currentPhase.String()))

		if err = safe.WriterModify[*omni.SecretRotation](ctx, r, secretRotation,
			func(res *omni.SecretRotation) error {
				res.TypedSpec().Value.Phase = currentPhase

				updateBackups := func(backupCerts []*specs.ClusterSecretsSpec_Certs_CA, cert *specs.ClusterSecretsSpec_Certs_CA) []*specs.ClusterSecretsSpec_Certs_CA {
					if !slices.Contains(xslices.Map(backupCerts, func(t *specs.ClusterSecretsSpec_Certs_CA) string {
						return t.String()
					}), cert.String()) {
						backupCerts = append(backupCerts, cert)
						if len(backupCerts) > BackedUpRotatedSecretsLimit {
							backupCerts = backupCerts[1:]
						}
					}

					return backupCerts
				}

				if currentPhase == specs.SecretRotationSpec_POST_ROTATE {
					switch res.TypedSpec().Value.Component {
					case specs.SecretRotationSpec_TALOS_CA:
						res.TypedSpec().Value.BackupCertsOs = updateBackups(res.TypedSpec().Value.BackupCertsOs, res.TypedSpec().Value.Certs.Os)
						res.TypedSpec().Value.Certs.Os = res.TypedSpec().Value.ExtraCerts.Os
						res.TypedSpec().Value.ExtraCerts.Os = nil
					case specs.SecretRotationSpec_KUBERNETES_CA:
						res.TypedSpec().Value.BackupCertsK8S = updateBackups(res.TypedSpec().Value.BackupCertsK8S, res.TypedSpec().Value.Certs.K8S)
						res.TypedSpec().Value.Certs.K8S = res.TypedSpec().Value.ExtraCerts.K8S
						res.TypedSpec().Value.ExtraCerts.K8S = nil
					case specs.SecretRotationSpec_NONE:
						// nothing to do
					}
				}

				return nil
			}); err != nil {
			return err
		}

		return nil
	}
}

func (s *Rotator) startCARotation(
	rotationStatus *omni.ClusterSecretsRotationStatus,
	cmSecretsMap map[resource.ID]*omni.ClusterMachineSecrets,
	rotatingComponent specs.SecretRotationSpec_Component,
	rotateRequestVersion string,
	generateCA func() (ca *x509.CertificateAuthority, err error),
	saveResult func(rotatingComponent specs.SecretRotationSpec_Component, ca *x509.CertificateAuthority) error,
) error {
	nextPhase := specs.SecretRotationSpec_PRE_ROTATE
	rotationStatus.TypedSpec().Value.Component = rotatingComponent
	rotationStatus.TypedSpec().Value.Phase = nextPhase
	rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("rotation phase %s %d/%d", nextPhase.String(), 0, len(cmSecretsMap))
	rotationStatus.TypedSpec().Value.Step = "starting secret rotation"

	switch rotatingComponent {
	case specs.SecretRotationSpec_TALOS_CA:
		rotationStatus.Metadata().Annotations().Set(omni.RotateTalosCAVersion, rotateRequestVersion)
	case specs.SecretRotationSpec_KUBERNETES_CA:
		rotationStatus.Metadata().Annotations().Set(omni.RotateKubernetesCAVersion, rotateRequestVersion)
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}

	newCA, err := generateCA()
	if err != nil {
		return fmt.Errorf("failed to generate new %s: %w", rotatingComponent.String(), err)
	}

	if err = saveResult(rotatingComponent, newCA); err != nil {
		return err
	}

	return nil
}

func (s *Rotator) handleClusterMachineScaling(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	clusterSecrets *omni.ClusterSecrets,
	secretRotation *omni.SecretRotation,
	cmStatusesMap map[resource.ID]*omni.ClusterMachineStatus,
	cmSecretsMap map[resource.ID]*omni.ClusterMachineSecrets,
	previousPhase, currentPhase specs.SecretRotationSpec_Phase,
) error {
	secretsToCreate := s.toCreate(cmStatusesMap, cmSecretsMap)
	for _, cmStatus := range secretsToCreate {
		_, isCP := cmStatus.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		rotation := &specs.ClusterMachineSecretsSpec_Rotation{
			Status:     specs.SecretRotationSpec_IDLE,
			Component:  specs.SecretRotationSpec_NONE,
			Phase:      previousPhase,
			ExtraCerts: &specs.ClusterSecretsSpec_Certs{},
		}

		secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
		if err != nil {
			return fmt.Errorf("failed to unmarshal secrets bundle: %w", err)
		}

		if isCP {
			// Marking the control plane node as in progress because we want to immediately start processing it
			rotation.Status = specs.SecretRotationSpec_IN_PROGRESS
			rotation.SecretRotationVersion = secretRotation.Metadata().Version().String()
			s.prepareCMSecretsForSecretRotation(secretRotation, secretsBundle, rotation, currentPhase)
		}

		if err = s.modifyClusterMachineSecret(ctx, r, logger, secretsBundle, cmStatus, rotation); err != nil {
			return err
		}
	}

	secretsToDestroy := s.toDestroy(cmStatusesMap, cmSecretsMap)
	for _, clusterMachineStatus := range secretsToDestroy {
		if err := s.destroyClusterMachineSecret(ctx, r, logger, clusterMachineStatus.Metadata().ID()); err != nil {
			return err
		}
	}

	if len(secretsToCreate) > 0 || len(secretsToDestroy) > 0 {
		// Created/Deleted ClusterMachineSecrets. Wait for a bit for them to be processed before moving on with rotation.
		return fmt.Errorf("waiting for ClusterMachineSecrets to be created/deleted: %w", sequence.ErrWait)
	}

	return nil
}

func (s *Rotator) handleClusterMachineSecretRotation(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	rotationStatus *omni.ClusterSecretsRotationStatus,
	secretRotation *omni.SecretRotation,
	cmSecretsMap map[resource.ID]*omni.ClusterMachineSecrets,
	cmStatusesMap map[resource.ID]*omni.ClusterMachineStatus,
	currentPhase specs.SecretRotationSpec_Phase,
) error {
	ongoingRotations := secretrotation.Candidates{}
	pendingRotations := secretrotation.Candidates{}

	clusterStatus, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, r, secretRotation.Metadata().ID())
	if err != nil {
		return err
	}

	lbConfig, err := safe.ReaderGetByID[*omni.LoadBalancerConfig](ctx, r, secretRotation.Metadata().ID())
	if err != nil {
		return err
	}

	for machineID, cmSecrets := range cmSecretsMap {
		if err = s.processMachine(ctx, r, logger, lbConfig, cmStatusesMap[machineID], cmSecrets, &pendingRotations, &ongoingRotations, currentPhase,
			secretRotation.Metadata().Version().String()); err != nil {
			return err
		}
	}

	if _, clusterLocked := clusterStatus.Metadata().Annotations().Get(omni.ClusterLocked); clusterLocked {
		rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("%s at phase %s", RotationPaused, rotationStatus.TypedSpec().Value.Phase.String())
		rotationStatus.TypedSpec().Value.Step = "waiting for the cluster to be unlocked"
		rotationStatus.TypedSpec().Value.Error = ""

		return fmt.Errorf("waiting for the cluster to be unlocked: %w", sequence.ErrWait)
	}

	viableCandidates, blockedCandidates := pendingRotations.Viable(secretrotation.Serial, secretrotation.Parallel)

	logger.Info("Rotating secret",
		zap.String("cluster", clusterStatus.Metadata().ID()),
		zap.String("component", rotationStatus.TypedSpec().Value.Component.String()),
		zap.String("phase", rotationStatus.TypedSpec().Value.Phase.String()),
		zap.Int("pending", pendingRotations.Len()),
		zap.Int("ongoing", ongoingRotations.Len()),
		zap.Int("pending_viable", len(viableCandidates)),
		zap.Int("pending_blocked", len(blockedCandidates)),
		zap.Int("locked", len(pendingRotations.Locked())),
		zap.Int("not_ready", len(pendingRotations.NotReady())),
	)

	if ongoingRotations.Len() > 0 {
		logger.Info("waiting for all ongoing rotations to be completed", zap.Strings("machines", xslices.Map(ongoingRotations.Candidates,
			func(res secretrotation.Candidate) string {
				return res.MachineID
			})))

		rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("rotation phase %s %d/%d",
			currentPhase, len(cmSecretsMap)-pendingRotations.Len(), len(cmSecretsMap))
		rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("rotating secret for machines: [%v]",
			s.hostnamesToString(xslices.Map(ongoingRotations.Candidates, func(res secretrotation.Candidate) string {
				return res.Hostname
			})))

		// We want to requeue here because we can only manually check if the rotation is valid. Rotation being successful does not create a meaningful event which can easily be tracked from Omni.
		return controller.NewRequeueInterval(time.Second)
	}

	if !clusterStatus.TypedSpec().Value.Ready || clusterStatus.TypedSpec().Value.Phase != specs.ClusterStatusSpec_RUNNING {
		rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("%s at phase %s", RotationPaused, rotationStatus.TypedSpec().Value.Phase.String())
		rotationStatus.TypedSpec().Value.Step = "waiting for the cluster to become ready"
		rotationStatus.TypedSpec().Value.Error = ""

		return fmt.Errorf("waiting for the cluster to be ready: %w", sequence.ErrWait)
	}

	if pendingRotations.Len() > 0 {
		if len(viableCandidates) > 0 {
			for _, candidate := range viableCandidates {
				if err := s.rotateClusterMachineSecrets(ctx, r, logger, cmSecretsMap[candidate.MachineID], secretRotation, cmStatusesMap[candidate.MachineID], currentPhase); err != nil {
					return err
				}
			}

			rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("rotation phase %s %d/%d",
				currentPhase, len(cmSecretsMap)-pendingRotations.Len(), len(cmSecretsMap))
			rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("rotating secret for machines: [%v]", s.hostnamesToString(xslices.Map(viableCandidates, func(res secretrotation.Candidate) string {
				return res.Hostname
			})))
			rotationStatus.TypedSpec().Value.Error = ""
		} else {
			rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("%s at phase %s", RotationPaused, rotationStatus.TypedSpec().Value.Phase.String())
			rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("waiting for machines: [%v]", s.hostnamesToString(xslices.Map(blockedCandidates, func(res secretrotation.Candidate) string {
				return res.Hostname
			})))
			rotationStatus.TypedSpec().Value.Error = ""
		}

		return fmt.Errorf("waiting for pending candidates: %w", sequence.ErrWait)
	}

	return nil
}

func (s *Rotator) destroyClusterMachineSecret(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineID resource.ID) error {
	destroyed, err := helpers.TeardownAndDestroy(ctx, r, omni.NewClusterMachineSecrets(machineID).Metadata())
	if err != nil {
		return err
	}

	if !destroyed {
		return nil
	}

	return r.RemoveFinalizer(ctx, omni.NewClusterMachineStatus(machineID).Metadata(), RotationStatusControllerName)
}

func (s *Rotator) modifyClusterMachineSecret(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, secretsBundle *talossecrets.Bundle,
	clusterMachineStatus *omni.ClusterMachineStatus, rotation *specs.ClusterMachineSecretsSpec_Rotation,
) error {
	data, err := json.Marshal(secretsBundle)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets bundle: %w", err)
	}

	if err = safe.WriterModify[*omni.ClusterMachineSecrets](ctx, r, omni.NewClusterMachineSecrets(clusterMachineStatus.Metadata().ID()),
		func(res *omni.ClusterMachineSecrets) error {
			helpers.CopyLabels(clusterMachineStatus, res, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
			res.TypedSpec().Value.Data = data
			res.TypedSpec().Value.Rotation = rotation

			return nil
		}); err != nil {
		return err
	}

	if err = r.AddFinalizer(ctx, omni.NewClusterMachineStatus(clusterMachineStatus.Metadata().ID()).Metadata(), RotationStatusControllerName); err != nil {
		return err
	}

	return nil
}

func (s *Rotator) rotateClusterMachineSecrets(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	cmSecrets *omni.ClusterMachineSecrets,
	secretRotation *omni.SecretRotation,
	cmStatus *omni.ClusterMachineStatus,
	currentPhase specs.SecretRotationSpec_Phase,
) error {
	rotation := &specs.ClusterMachineSecretsSpec_Rotation{
		Status:                specs.SecretRotationSpec_IN_PROGRESS,
		Phase:                 currentPhase,
		ExtraCerts:            &specs.ClusterSecretsSpec_Certs{},
		SecretRotationVersion: secretRotation.Metadata().Version().String(),
	}

	secretsBundle, err := omni.ToSecretsBundle(cmSecrets.TypedSpec().Value.Data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal secrets bundle: %w", err)
	}

	s.prepareCMSecretsForSecretRotation(secretRotation, secretsBundle, rotation, currentPhase)

	logger.Info("rotating machine secret", zap.String("machine", cmStatus.Metadata().ID()),
		zap.String("phase", rotation.Phase.String()), zap.String("component", rotation.Component.String()))

	if err = s.modifyClusterMachineSecret(ctx, r, logger, secretsBundle, cmStatus, rotation); err != nil {
		return err
	}

	return nil
}

func (s *Rotator) prepareCMSecretsForSecretRotation(
	secretRotation *omni.SecretRotation,
	secretsBundle *talossecrets.Bundle,
	rotation *specs.ClusterMachineSecretsSpec_Rotation,
	currentPhase specs.SecretRotationSpec_Phase,
) {
	switch secretRotation.TypedSpec().Value.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		s.prepareTalosCARotation(secretRotation, secretsBundle, rotation, currentPhase)
	case specs.SecretRotationSpec_KUBERNETES_CA:
		s.prepareKubernetesCARotation(secretRotation, secretsBundle, rotation, currentPhase)
	case specs.SecretRotationSpec_NONE:
		// nothing to do
	}
}

//nolint:dupl
func (s *Rotator) prepareTalosCARotation(
	secretRotation *omni.SecretRotation,
	secretsBundle *talossecrets.Bundle,
	rotation *specs.ClusterMachineSecretsSpec_Rotation,
	currentPhase specs.SecretRotationSpec_Phase,
) {
	if secretRotation.TypedSpec().Value.ExtraCerts.GetOs() == nil {
		return
	}

	rotation.Component = specs.SecretRotationSpec_TALOS_CA
	rotation.Phase = currentPhase

	switch currentPhase {
	case specs.SecretRotationSpec_OK:
		// nothing to do
	case specs.SecretRotationSpec_PRE_ROTATE:
		rotation.ExtraCerts.Os = &specs.ClusterSecretsSpec_Certs_CA{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.Os.Key,
		}
	case specs.SecretRotationSpec_ROTATE:
		secretsBundle.Certs.OS = &x509.PEMEncodedCertificateAndKey{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.Os.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.Os.Key,
		}
		rotation.ExtraCerts.Os = &specs.ClusterSecretsSpec_Certs_CA{
			Crt: secretRotation.TypedSpec().Value.Certs.Os.Crt,
			Key: secretRotation.TypedSpec().Value.Certs.Os.Key,
		}
	case specs.SecretRotationSpec_POST_ROTATE:
		rotation.ExtraCerts = nil
	}
}

//nolint:dupl
func (s *Rotator) prepareKubernetesCARotation(
	secretRotation *omni.SecretRotation,
	secretsBundle *talossecrets.Bundle,
	rotation *specs.ClusterMachineSecretsSpec_Rotation,
	currentPhase specs.SecretRotationSpec_Phase,
) {
	if secretRotation.TypedSpec().Value.ExtraCerts.GetK8S() == nil {
		return
	}

	rotation.Component = specs.SecretRotationSpec_KUBERNETES_CA
	rotation.Phase = currentPhase

	switch currentPhase {
	case specs.SecretRotationSpec_OK:
		// nothing to do
	case specs.SecretRotationSpec_PRE_ROTATE:
		rotation.ExtraCerts.K8S = &specs.ClusterSecretsSpec_Certs_CA{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Key,
		}
	case specs.SecretRotationSpec_ROTATE:
		secretsBundle.Certs.K8s = &x509.PEMEncodedCertificateAndKey{
			Crt: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Crt,
			Key: secretRotation.TypedSpec().Value.ExtraCerts.K8S.Key,
		}
		rotation.ExtraCerts.K8S = &specs.ClusterSecretsSpec_Certs_CA{
			Crt: secretRotation.TypedSpec().Value.Certs.K8S.Crt,
			Key: secretRotation.TypedSpec().Value.Certs.K8S.Key,
		}
	case specs.SecretRotationSpec_POST_ROTATE:
		rotation.ExtraCerts = nil
	}
}

func (s *Rotator) processMachine(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	lbConfig *omni.LoadBalancerConfig,
	cmStatus *omni.ClusterMachineStatus,
	cmSecret *omni.ClusterMachineSecrets,
	pendingRotations *secretrotation.Candidates,
	ongoingRotations *secretrotation.Candidates,
	currentPhase specs.SecretRotationSpec_Phase,
	secretRotationVersion string,
) error {
	_, isControlPlane := cmStatus.Metadata().Labels().Get(omni.LabelControlPlaneRole)
	_, locked := cmStatus.Metadata().Annotations().Get(omni.MachineLocked)
	hostname, _ := cmStatus.Metadata().Labels().Get(omni.LabelHostname)

	candidate := secretrotation.Candidate{
		MachineID:               cmStatus.Metadata().ID(),
		Hostname:                hostname,
		ControlPlane:            isControlPlane,
		Locked:                  locked,
		Ready:                   cmStatus.TypedSpec().Value.Ready,
		RemoteGeneratorFactory:  s.RemoteGeneratorFactory,
		KubernetesClientFactory: s.KubernetesClientFactory,
	}

	// ClusterMachineSecrets hasn't been updated yet to match the current rotation phase, update it
	if cmSecret.TypedSpec().Value.GetRotation() == nil || currentPhase != cmSecret.TypedSpec().Value.Rotation.Phase {
		pendingRotations.Add(candidate)

		return nil
	}

	// ClusterMachineSecretsRotation is in progress, validate if the rotation phase is completed
	if cmSecret.TypedSpec().Value.Rotation.Status == specs.SecretRotationSpec_IN_PROGRESS {
		if cmSecret.TypedSpec().Value.Rotation.GetSecretRotationVersion() != secretRotationVersion {
			logger.Warn("secret rotation version mismatch. defer validation.", zap.String("machine", candidate.MachineID),
				zap.String("version", cmSecret.TypedSpec().Value.Rotation.GetSecretRotationVersion()))

			ongoingRotations.Add(candidate)

			return nil
		}

		logger.Info("validating secret rotation", zap.String("machine", candidate.MachineID), zap.String("phase", currentPhase.String()))

		valid, err := candidate.Validate(ctx, lbConfig, cmStatus, cmSecret)
		if err != nil {
			logger.Error("failed to validate secret rotation", zap.String("machine", candidate.MachineID), zap.Error(err))
		}

		if valid {
			if err = safe.WriterModify[*omni.ClusterMachineSecrets](ctx, r, cmSecret,
				func(res *omni.ClusterMachineSecrets) error {
					res.TypedSpec().Value.Rotation.Status = specs.SecretRotationSpec_IDLE

					return nil
				}); err != nil {
				return err
			}

			return nil
		}

		ongoingRotations.Add(candidate)
	}

	return nil
}

func (s *Rotator) getCMStatusesAndSecrets(ctx context.Context, r controller.Reader, clusterID resource.ID) (
	map[resource.ID]*omni.ClusterMachineStatus, map[resource.ID]*omni.ClusterMachineSecrets, error,
) {
	statuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return nil, nil, err
	}

	secrets, err := safe.ReaderListAll[*omni.ClusterMachineSecrets](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return nil, nil, err
	}

	secretsMap := xslices.ToMap(slices.Collect(secrets.All()), func(r *omni.ClusterMachineSecrets) (resource.ID, *omni.ClusterMachineSecrets) {
		return r.Metadata().ID(), r
	})
	statusesMap := xslices.ToMap(slices.Collect(statuses.All()), func(r *omni.ClusterMachineStatus) (resource.ID, *omni.ClusterMachineStatus) {
		return r.Metadata().ID(), r
	})

	return statusesMap, secretsMap, nil
}

func (s *Rotator) toCreate(cmStatusesMap map[resource.ID]*omni.ClusterMachineStatus, cmSecretsMap map[resource.ID]*omni.ClusterMachineSecrets) map[resource.ID]*omni.ClusterMachineStatus {
	items := map[resource.ID]*omni.ClusterMachineStatus{}

	for machineID, clusterMachineStatus := range cmStatusesMap {
		if _, ok := cmSecretsMap[machineID]; !ok && clusterMachineStatus.Metadata().Phase() == resource.PhaseRunning {
			items[machineID] = clusterMachineStatus
		}
	}

	return items
}

func (s *Rotator) toDestroy(cmStatusesMap map[resource.ID]*omni.ClusterMachineStatus, cmSecretsMap map[resource.ID]*omni.ClusterMachineSecrets) map[resource.ID]*omni.ClusterMachineStatus {
	items := map[resource.ID]*omni.ClusterMachineStatus{}

	for machineID, clusterMachineStatus := range cmStatusesMap {
		if _, ok := cmSecretsMap[machineID]; ok && clusterMachineStatus.Metadata().Phase() == resource.PhaseTearingDown {
			items[machineID] = clusterMachineStatus
		}
	}

	return items
}

func (s *Rotator) hostnamesToString(hostnames []string) string {
	slices.Sort(hostnames)

	if len(hostnames) > 2 {
		return fmt.Sprintf("%s, %d more", strings.Join(hostnames[:2], ", "), len(hostnames)-2)
	}

	return strings.Join(hostnames, ", ")
}

func (s *Rotator) getSecretRotation(ctx context.Context, r controller.ReaderWriter, id resource.ID) (*omni.SecretRotation, error) {
	uncachedReader, ok := r.(controller.UncachedReader)
	if !ok {
		return nil, fmt.Errorf("reader does not support uncached reads")
	}

	res, err := uncachedReader.GetUncached(ctx, omni.NewSecretRotation(id).Metadata())
	if err != nil {
		return nil, fmt.Errorf("error getting secret rotation: %w", err)
	}

	resTyped, ok := res.(*omni.SecretRotation)
	if !ok {
		return nil, fmt.Errorf("unexpected resource type: %T", res)
	}

	return resTyped, nil
}
