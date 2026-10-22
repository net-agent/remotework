package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// LoadJSONFile 加载 JSON/JSONC 文件到对象里。
//
// 支持 // 行注释（含行尾注释）与 /* */ 块注释；注释由字符串感知的
// stripJSONCComments 去除，因此字符串内的 "http://..." 不会被误删。
// 解析时拒绝结构体中不存在的字段（DisallowUnknownFields），避免拼写错误
// 或已废弃字段被静默忽略。
func LoadJSONFile(pathname string, v interface{}) error {
	buf, err := os.ReadFile(pathname)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(stripJSONCComments(buf)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("parse json file %q: %w", pathname, err)
	}

	// 只允许一个 JSON 值；尾随的非空白内容视为格式错误
	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("parse json file %q: unexpected trailing data", pathname)
		}
		return fmt.Errorf("parse json file %q: %w", pathname, err)
	}
	return nil
}

// stripJSONCComments 返回去除 JSONC 注释后的字节切片。
//
// 扫描时区分字符串字面量与转义字符，只把字符串之外的注释内容替换为空格；
// 换行符原样保留，因此返回值的长度与换行位置和输入一致，JSON 解码错误
// 中的行列号仍然可用。
func stripJSONCComments(src []byte) []byte {
	out := append([]byte(nil), src...)

	const (
		stateNormal = iota
		stateString
		stateLineComment
		stateBlockComment
	)

	state := stateNormal
	escaped := false
	for i := 0; i < len(out); i++ {
		c := out[i]

		switch state {
		case stateNormal:
			switch {
			case c == '"':
				state = stateString
				escaped = false
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				state = stateLineComment
				out[i], out[i+1] = ' ', ' '
				i++
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				state = stateBlockComment
				out[i], out[i+1] = ' ', ' '
				i++
			}

		case stateString:
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				state = stateNormal
			}

		case stateLineComment:
			if c == '\n' || c == '\r' {
				state = stateNormal
				continue
			}
			out[i] = ' '

		case stateBlockComment:
			switch {
			case c == '*' && i+1 < len(out) && out[i+1] == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				state = stateNormal
			case c == '\n' || c == '\r':
				// 保留换行以维持行号
			default:
				out[i] = ' '
			}
		}
	}

	return out
}

// LoadTomlFile 加载 TOML 文件到对象里；拒绝无法映射到结构体字段的键。
func LoadTomlFile(pathname string, v interface{}) error {
	md, err := toml.DecodeFile(pathname, v)
	if err != nil {
		return err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return fmt.Errorf("parse toml file %q: unknown field(s): %s",
			pathname, strings.Join(keys, ", "))
	}
	return nil
}

// LoadConfigFile 根据文件扩展名（.json/.toml）加载配置到目标对象
func LoadConfigFile(configFile string, v interface{}) error {
	switch strings.ToLower(filepath.Ext(configFile)) {
	case ".json":
		return LoadJSONFile(configFile, v)
	case ".toml":
		return LoadTomlFile(configFile, v)
	default:
		return fmt.Errorf("config file [%s] not supported, must be json or toml", configFile)
	}
}

// statFile 判断路径是否存在且为普通文件。除“不存在”之外的 stat 错误
// （例如权限不足）会原样返回，由调用方决定如何呈现。
func statFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// FileExist 判断路径是否存在且为普通文件。
// 该函数不会因权限等错误终止进程；需要区分具体错误的调用方应使用内部
// statFile（ResolveConfigFile 即如此）。
func FileExist(path string) bool {
	ok, err := statFile(path)
	return err == nil && ok
}

// FirstString 返回第一个长度大于0的字符串
func FirstString(strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return ""
}

// ResolveConfigFile 解析配置文件路径，支持 fallback 到 config.json / config.toml。
// 使用 filepath 处理平台路径分隔符；stat 失败（非“不存在”）时返回错误，
// 不再静默回退或终止进程。
func ResolveConfigFile(configName string) (string, error) {
	ok, err := statFile(configName)
	if err != nil {
		return "", fmt.Errorf("stat config file %q: %w", configName, err)
	}
	if ok {
		return configName, nil
	}

	dir := filepath.Dir(configName)
	candidates := []string{
		filepath.Join(dir, "config.json"),
		filepath.Join(dir, "config.toml"),
	}
	for _, candidate := range candidates {
		ok, err := statFile(candidate)
		if err != nil {
			return "", fmt.Errorf("stat config file %q: %w", candidate, err)
		}
		if ok {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("config file not found: tried '%v', '%v', '%v'",
		configName, candidates[0], candidates[1])
}
