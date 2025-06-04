// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-tail"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// NewLogHandler returns a new LogHandler.
func NewLogHandler(machineMap *MachineMap, omniState state.State, storageConfig *config.LogsMachine, logger *zap.Logger) (*LogHandler, error) {
	cache, err := NewMachineCache(storageConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine cache: %w", err)
	}

	handler := LogHandler{
		Map:       machineMap,
		OmniState: omniState,
		Cache:     cache,
		logger:    logger,
	}

	return &handler, nil
}

// LogHandler stores a map of machines to their circular log buffers.
type LogHandler struct {
	OmniState state.State
	Map       *MachineMap
	logger    *zap.Logger
	Cache     *MachineCache
}

// Start starts the LogHandler.
func (h *LogHandler) Start(ctx context.Context) error {
	h.logger.Info("starting log handler")

	eventCh := make(chan state.Event)

	if err := h.OmniState.WatchKind(
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
			if err := h.Cache.Close(); err != nil {
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

				h.Map.RemoveByMachineID(machineID)

				err := h.Cache.Remove(machineID)
				if err != nil {
					h.logger.Error("failed to remove machine buffer", zap.String("machine_id", string(machineID)), zap.Error(err))
				}
			}
		}
	}
}

// HasLink checks if the machine src address can be resolved into a siderolink.Link.
func (h *LogHandler) HasLink(srcAddress netip.Addr) bool {
	ip := srcAddress.String()
	if ip == "" {
		return false
	}

	_, err := h.Map.GetMachineID(ip)

	return err == nil
}

// HandleMessage handles a log message.
func (h *LogHandler) HandleMessage(srcAddress netip.Addr, rawData []byte) {
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

	err := h.writeMessage(currentIP, rawData)
	if err != nil {
		logger.Error("failed to write message to buffer", zap.Error(err))

		return
	}
}

func (h *LogHandler) writeMessage(ip string, data []byte) error {
	id, err := h.Map.GetMachineID(ip)
	if err != nil {
		return fmt.Errorf("failed to get machine ID for ip address '%s': %w", ip, err)
	}

	err = h.Cache.WriteMessage(id, data)
	if err != nil {
		return fmt.Errorf("failed to write message to buffer for machine '%s': %w", id, err)
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

	id, err := h.Map.GetMachineID(currentIP)
	if err != nil {
		h.logger.Error("failed to get machine ID for ip address", zap.String("ip", currentIP), zap.Error(err))

		return
	}

	logger = logger.With(zap.String("machine_id", string(id)))

	logger.Error("error from the log server", zap.Error(hErr))
}

// GetReader returns a line reader for the given machine ID.
func (h *LogHandler) GetReader(machineID MachineID, follow bool, tailLines optional.Optional[int32]) (*LineReader, error) {
	buf, err := h.Cache.GetBuffer(machineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buffer for machine '%s': %w", machineID, err)
	}

	var r interface {
		io.ReadCloser
		io.Seeker
	}

	if follow {
		r = buf.GetStreamingReader()
	} else {
		r = buf.GetReader()
	}

	if tailLines.IsPresent() {
		// since we are surrounding each message with \n we should increase lines by two times.
		lines := int(tailLines.ValueOrZero()) * 2

		err := tail.SeekLines(r, lines)
		if err != nil {
			return nil, fmt.Errorf("failed to seek %d lines: %w", lines, err)
		}
	}

	return &LineReader{reader: r}, nil
}

// LineReader is a reader which reads lines surrounded by \n from the underlying reader.
type LineReader struct {
	buf    *bufio.Reader
	reader io.ReadCloser
}

// Close closes the LineReader underlying reader.
func (r *LineReader) Close() error {
	return r.reader.Close()
}

// ReadLine reads a line from the underlying reader.
func (r *LineReader) ReadLine() ([]byte, error) {
	if r.buf == nil {
		r.buf = bufio.NewReader(r.reader)
	}

	for {
		emptyLine, err := r.buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		if len(emptyLine) != 1 {
			// missed the start of the line, skipping to the next entry
			continue
		}

		logLine, err := r.buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		return trimNewlines(logLine), nil
	}
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
