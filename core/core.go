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
func (s *Instance) Start() error {
	s.access.Lock()
	defer s.access.Unlock()

	s.running = true
	for _, f := range s.features {
		if err := f.Start(); err != nil {
			return newError("failed to start feature: ", err)
		}
	}

	return nil
}

// Close stops the Xray instance and all registered features.
func (s *Instance) Close() error {
	s.access.Lock()
	defer s.access.Unlock()

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

// RequireFeatures is a helper to retrieve required features from the instance.
func (s *Instance) RequireFeatures(callback interface{}) error {
	callbackType := reflect.TypeOf(callback)
	if callbackType.Kind() != reflect.Func {
		return newError("not a function")
	}

	var args []reflect.Value
	for i := 0; i < callbackType.NumIn(); i++ {
		featureType := callbackType.In(i)
		f := s.GetFeature(reflect.New(featureType).Interface())
		if f == nil {
			return newError("feature not found: ", featureType)
		}
		args = append(args, reflect.ValueOf(f))
	}

	reflect.ValueOf(callback).Call(args)
	return nil
}

// Convenience accessors for common features.

// DNSClient returns the DNS client feature of the instance.
func (s *Instance) DNSClient() dns.Client {
	return s.GetFeature(dns.ClientType()).(dns.Client)
}

// PolicyManager returns the policy manager feature of the instance.
func (s *Instance) PolicyManager() policy.Manager {
	return s.GetFeature(policy.ManagerType()).(policy.Manager)
}

// Router returns the routing feature of the instance.
func (s *Instance) Router() routing.Router {
	return s.GetFeature(routing.RouterType()).(routing.Router)
}

// InboundHandlerManager returns the inbound handler manager feature.
func (s *Instance) InboundHandlerManager() inbound.Manager {
	return s.GetFeature(inbound.ManagerType()).(inbound.Manager)
}

// OutboundHandlerManager returns the outbound handler manager feature.
func (s *Instance) OutboundHandlerManager() outbound.Manager {
	return s.GetFeature(outbound.ManagerType()).(outbound.Manager)
}

// StatsManager returns the stats manager feature of the instance.
func (s *Instance) StatsManager() stats.Manager {
	return s.GetFeature(stats.ManagerType()).(stats.Manager)
}
