// Package core provides the core functionality of Xray-core.
// It manages the lifecycle of the Xray instance, including
// initialization, starting, and stopping of all registered features.
package core

import (
	"context"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/features"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/features/stats"
)

// Version is the current version of Xray-core.
const Version = "1.8.0"

// Instance represents a running Xray instance with all its features.
type Instance struct {
	access   sync.Mutex
	features []features.Feature
	featureIndex map[string]features.Feature
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Xray instance with the given configuration.
func New(config *Config) (*Instance, error) {
	var s = &Instance{
		featureIndex: make(map[string]features.Feature),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if config == nil {
		return nil, newError("config is nil")
	}

	if err := s.addInboundHandlers(config); err != nil {
		return nil, err
	}

	return s, nil
}

// AddFeature registers a new feature to the instance.
// Returns an error if the instance is already running.
func (s *Instance) AddFeature(feature features.Feature) error {
	s.access.Lock()
	defer s.access.Unlock()

	if s.running {
		return newError("cannot add feature to a running instance")
	}

	s.features = append(s.features, feature)
	s.featureIndex[serial.GetMessageType(feature)] = feature
	return nil
}

// GetFeature returns the registered feature of the given type.
func (s *Instance) GetFeature(featureType interface{}) features.Feature {
	s.access.Lock()
	defer s.access.Unlock()
	return s.featureIndex[serial.GetMessageType(featureType)]
}

// Start starts the Xray instance and all registered features.
// Features are started in the order they were registered.
func (s *Instance) Start() error {
	s.access.Lock()
	defer s.access.Unlock()

	s.running = true
	for _, f := range s.features {
		if err := f.Start(); err != nil {
			// Stop already-started features before returning the error
			// to avoid leaving the instance in a partially started state.
			// TODO: implement rollback to stop features that already started successfully.
			return newError("failed to start feature: ", err)
		}
	}

	return nil
}

// Close stops the Xray instance and all registered features.
// All features are closed even if some return errors; all errors are collected.
func (s *Instance) Close() error {
	s.access.Lock()
	defer s.access.Unlock()

	if !s.running {
		// Already closed or never started; nothing to do.
		return nil
	}

	s.running = false
	s.cancel()

	var errs []error
	for _, f := range s.features {
		if err := common.Close(f); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return newError("failed to close some features").Base(errs[0])
	}
	return nil
}

// Requi
