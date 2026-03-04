package configv2

import "github.com/net-agent/remotework/utils"

// LoadFile loads a raw configuration from the given file path.
// Supports JSON (with // comments) and TOML formats.
func LoadFile(path string) (*RawConfig, error) {
	cfg := &RawConfig{}
	if err := utils.LoadConfigFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadAndNormalize loads, normalizes, and validates a configuration file.
// Returns the canonical config ready for the runtime layer.
func LoadAndNormalize(path string) (*CanonicalConfig, error) {
	raw, err := LoadFile(path)
	if err != nil {
		return nil, err
	}
	canonical, err := Normalize(raw)
	if err != nil {
		return nil, err
	}
	if err := Validate(canonical); err != nil {
		return nil, err
	}
	return canonical, nil
}
