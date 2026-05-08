package core

import (
	"io"
	"strings"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"google.golang.org/protobuf/proto"
)

// ConfigFormat defines the format of a config file.
type ConfigFormat struct {
	Name      string
	Extension []string
	Loader    ConfigLoader
}

// ConfigLoader is a function to load config from a reader with optional environment.
type ConfigLoader func(input []*TypedReader) (*Config, error)

// TypedReader is a reader with an associated format type.
type TypedReader struct {
	Reader io.Reader
	Format string
}

var configFormatRegistry = make(map[string]*ConfigFormat)

// RegisterConfigLoader registers a new config loader for a given format.
// This should be called during initialization.
func RegisterConfigLoader(format *ConfigFormat) error {
	name := strings.ToLower(format.Name)
	if _, found := configFormatRegistry[name]; found {
		return errors.New("config format already registered: ", name)
	}
	configFormatRegistry[name] = format
	for _, ext := range format.Extension {
		ext = strings.ToLower(ext)
		if _, found := configFormatRegistry[ext]; found {
			// Log a warning but don't fail — extension conflicts can happen with aliases.
			continue
		}
		configFormatRegistry[ext] = format
	}
	return nil
}

// GetConfigFormat returns the registered config format by name or extension.
func GetConfigFormat(name string) (*ConfigFormat, error) {
	name = strings.ToLower(name)
	if f, found := configFormatRegistry[name]; found {
		return f, nil
	}
	return nil, errors.New("unknown config format: ", name)
}

// LoadConfig loads a Config from the given readers, detecting format from the
// provided format name (e.g., "json", "pb", "toml").
func LoadConfig(formatName string, input []*TypedReader) (*Config, error) {
	format, err := GetConfigFormat(formatName)
	if err != nil {
		return nil, err
	}
	return format.Loader(input)
}

// loadProtobufConfig reads and unmarshals a protobuf-encoded Config from r.
func loadProtobufConfig(r io.Reader) (*Config, error) {
	data, err := buf.ReadAllToBytes(r)
	if err != nil {
		return nil, errors.New("failed to read protobuf config").Base(err)
	}
	cfg := new(Config)
	if err := proto.Unmarshal(data, cfg); err != nil {
		return nil, errors.New("failed to unmarshal protobuf config").Base(err)
	}
	return cfg, nil
}

func init() {
	// Register the built-in protobuf config format.
	// Also register "proto" as a convenience alias for the protobuf format.
	// Note: "pb" is the canonical extension I use for binary configs generated
	// by my local tooling; keeping "protobuf" and "proto" as aliases for
	// compatibility with other tools in my workflow.
	// Also added "bin" as an alias since some of my scripts output .bin files.
	_ = RegisterConfigLoader(&ConfigFormat{
		Name:      "Protobuf",
		Extension: []string{"pb", "protobuf", "proto", "bin"},
		Loader: func(inputs []*TypedReader) (*Config, error) {
			if len(inputs) != 1 {
				return nil, errors.New("protobuf format requires exactly one input")
			}
			return loadProtobufConfig(inputs[0].Reader)
		},
	})
