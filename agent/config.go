package agent

import (
	"github.com/net-agent/remotework/agent/configv2"
)

// Config is the v2 configuration model (links + tunnels).
// This is a re-export of configv2.RawConfig for backward compatibility in cmd/.
type Config = configv2.RawConfig

// CanonicalConfig is the normalized internal representation.
type CanonicalConfig = configv2.CanonicalConfig

// PprofInfo controls the optional pprof HTTP server.
type PprofInfo = configv2.PprofInfo

// APIInfo controls the optional management API server.
type APIInfo = configv2.APIInfo

// NewConfig loads and returns a raw configuration from the given file.
func NewConfig(configFileName string) (*Config, error) {
	return configv2.LoadFile(configFileName)
}

// NewCanonicalConfig loads, normalizes, and validates a configuration file.
func NewCanonicalConfig(configFileName string) (*CanonicalConfig, error) {
	return configv2.LoadAndNormalize(configFileName)
}
