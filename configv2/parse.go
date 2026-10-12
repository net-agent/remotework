package configv2

import (
	"fmt"

	"github.com/net-agent/remotework/utils"
)

// LoadFile loads a raw configuration from the given file path.
// Supports JSON (with // comments) and TOML formats.
func LoadFile(path string) (*RawConfig, error) {
	cfg := &RawConfig{}
	if err := utils.LoadConfigFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadAndCheck 容忍式加载：逐项归一化并校验，返回可用的（可能是部分的）
// 配置与全部问题列表。仅当文件不可读或整体格式错误时 config 为 nil。
// 调用方据此实现"坏项跳过、好项照跑"的容错加载。
func LoadAndCheck(path string) (*CanonicalConfig, *ValidationErrors) {
	raw, err := LoadFile(path)
	if err != nil {
		errs := &ValidationErrors{}
		errs.add(ErrConfigParse, fmt.Sprintf("load config file: %v", err))
		return nil, errs
	}

	canonical, errs := normalizeAll(raw)
	if verr := Validate(canonical); verr != nil {
		if ve, ok := verr.(*ValidationErrors); ok {
			errs.Errors = append(errs.Errors, ve.Errors...)
		}
	}
	return canonical, errs
}
