package configv2

import (
	"fmt"

	"github.com/net-agent/remotework/utils"
)

// LoadFile loads a raw configuration from the given file path.
// Supports JSON (with // and /* */ comments) and TOML formats.
// 未知字段会被拒绝（JSON: DisallowUnknownFields；TOML: MetaData.Undecoded）。
func LoadFile(path string) (*RawConfig, error) {
	cfg := &RawConfig{}
	if err := utils.LoadConfigFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadAndCheck 容忍式加载：逐项归一化并校验，返回可用的（可能是部分的）
// 配置与全部问题列表。仅当文件不可读或整体格式错误时 config 为 nil。
//
// 坏 link/tunnel 会按 ValidationError.Scope/Item/ItemIndex 从 canonical 中
// 精确剔除；好项保留。跨项校验只在实际可用的条目之间进行，因此一个坏项
// 不会连带拖垮与其本不冲突的好项。错误仍完整返回，供 cmd/API 展示。
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
			pruneInvalidItems(canonical, ve.Errors)
		}
	}
	return canonical, errs
}

// pruneInvalidItems 删除被 Validate 判定为不可用的 link/tunnel。
// ScopeConfig 错误（如 pprof 监听非回环）不属于单项，保留配置并返回错误。
func pruneInvalidItems(cfg *CanonicalConfig, verrs []ValidationError) {
	dropLinks := make(map[string]struct{})
	dropTunnelIdx := make(map[int]struct{})

	for _, e := range verrs {
		switch e.Scope {
		case ScopeLink:
			dropLinks[e.Item] = struct{}{}
		case ScopeTunnel:
			if e.ItemIndex >= 0 && e.ItemIndex < len(cfg.Tunnels) {
				dropTunnelIdx[e.ItemIndex] = struct{}{}
			}
		}
	}

	for alias := range dropLinks {
		delete(cfg.Links, alias)
	}

	if len(dropTunnelIdx) == 0 {
		return
	}
	kept := make([]TunnelSpec, 0, len(cfg.Tunnels)-len(dropTunnelIdx))
	for i, t := range cfg.Tunnels {
		if _, drop := dropTunnelIdx[i]; !drop {
			kept = append(kept, t)
		}
	}
	cfg.Tunnels = kept
}
