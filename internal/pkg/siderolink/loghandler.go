// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

type LogStoreManager interface {
	Run(ctx context.Context) error
	Exists(ctx context.Context, id string) (bool, error)
	Create(id string) (logstore.LogStore, error)
	Remove(ctx context.Context, id string) error
}

// NewLogHandler returns a new LogHandler.
func NewLogHandler(secondaryStorageDB *sql.DB, machineMap *MachineMap, omniState state.State, storageConfig *config.LogsMachine, logger *zap.Logger) (*LogHandler, error) {
	cache, err := NewMachineCache(secondaryStorageDB, storageConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine cache: %w", err)
	}

	handler := LogHandler{
		machineMap: machineMap,
		omniState:  omniState,
		cache:      cache,
		logger:     logger,
	}

	return &handler, nil
}

// LogHandler stores a map of machines to their log stores.
type LogHandler struct {
	omniState  state.State
	machineMap *MachineMap
	logger     *zap.Logger
	cache      *MachineCache
}

// Start starts the LogHandler.
func (h *LogHandler) Start(ctx context.Context) error {
	h.logger.Info("starting log handler")

	eg, ctx := panichandler.ErrGroupWithContext(ctx)

	eg.Go(func() error {
		return h.cache.Run(ctx)
	})

	eg.Go(func() error {
		eventCh := make(chan state.Event)

		if err := h.omniState.WatchKind(
			ctx,
			omni.NewMachine(resources.DefaultNamespace, "").Metadata(),
			eventCh,
			state.WithBootstrapContents(true),
		); err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				if err := h.cache.Close(); err != nil {
					h.logger.Error("failed to close machine logs cache", zap.Error(err))
				}

				return nil
			case event := <-eventCh:
				switch event.Type {
				case state.Created, state.Updated, state.Bootstrapped, state.Noop:
					// ignore
				case state.Errored:
					return fmt.Errorf("error watching machines: %w", event.Error)
				case state.Destroyed:
					machineID := MachineID(event.Resource.Metadata().ID())

					h.machineMap.RemoveByMachineID(machineID)

					err := h.cache.remove(ctx, machineID)
					if err != nil {
						h.logger.Error("failed to remove machine log store", zap.String("machine_id", string(machineID)), zap.Error(err))
					}
				}
			}
		}
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait for log handler goroutines: %w", err)
	}

	return nil
}

// HasLink checks if the machine src address can be resolved into a siderolink.Link.
func (h *LogHandler) HasLink(srcAddress netip.Addr) bool {
	ip := srcAddress.String()
	if ip == "" {
		return false
	}

	_, err := h.machineMap.GetMachineID(ip)

	return err == nil
}

// HandleMessage handles a log message.
func (h *LogHandler) HandleMessage(ctx context.Context, srcAddress netip.Addr, rawData []byte) {
	currentIP := srcAddress.String()
	if currentIP == "" {
		h.logger.Error("empty IP address")

		return
	}

	logger := h.logger.With(zap.String("machine_ip", currentIP))
	rawData = trimNewlines(rawData)

	if len(rawData) == 0 {
		logger.Warn("empty log message")

		return
	}

	err := h.writeMessage(ctx, currentIP, rawData)
	if err != nil {
		logger.Error("failed to write message to log store", zap.Error(err))

		return
	}
}

func (h *LogHandler) writeMessage(ctx context.Context, ip string, data []byte) error {
	id, err := h.machineMap.GetMachineID(ip)
	if err != nil {
		return fmt.Errorf("failed to get machine ID for ip address %q: %w", ip, err)
	}

	err = h.cache.WriteMessage(ctx, id, data)
	if err != nil {
		return fmt.Errorf("failed to write message to log store for machine %q: %w", id, err)
	}

	return nil
}

// HandleError handles an error from the server.
func (h *LogHandler) HandleError(srcAddress netip.Addr, hErr error) {
	currentIP := srcAddress.String()
	if currentIP == "" {
		h.logger.Error("empty IP address")

		return
	}

	logger := h.logger.With(zap.String("machine_ip", currentIP))

	id, err := h.machineMap.GetMachineID(currentIP)
	if err != nil {
		h.logger.Error("failed to get machine ID for ip address", zap.String("ip", currentIP), zap.Error(err))

		return
	}

	logger = logger.With(zap.String("machine_id", string(id)))

	logger.Error("error from the log server", zap.Error(hErr))
}

// GetReader returns a line reader for the given machine ID.
func (h *LogHandler) GetReader(ctx context.Context, machineID MachineID, follow bool, tailLines optional.Optional[int32]) (logstore.LineReader, error) {
	logStore, err := h.cache.getLogStore(ctx, machineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log store for machine %q: %w", machineID, err)
	}

	nLines := tailLines.ValueOr(-1)

	reader, err := logStore.Reader(ctx, int(nLines), follow)
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for machine %q: %w", machineID, err)
	}

	return reader, nil
}

// trimNewlines trims a newline from the start and from end of a byte slice.
func trimNewlines(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	if data[0] == '\n' {
		data = data[1:]
	}

	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data
}
