package utils

import (
	"fmt"
	"os"
	"strings"
)

// ResolveSecret 解析密钥引用（secret reference）。
//
// 支持两种写法：
//   - "env:NAME"：从环境变量 NAME 读取；
//   - "NAME"：等价于 "env:NAME"，直接按环境变量名读取。
//
// ref 为空时返回空字符串与 nil（表示未配置，由调用方决定是否必需）。
// 引用未设置、为空或使用不支持的 scheme 时返回明确错误；调用方必须
// 失败关闭（fail closed），绝不能把引用字面量当作密钥使用。
func ResolveSecret(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}

	name := ref
	if scheme, rest, ok := strings.Cut(ref, ":"); ok {
		if !strings.EqualFold(scheme, "env") {
			return "", fmt.Errorf("unsupported secret reference %q: only env:NAME is supported", ref)
		}
		name = strings.TrimSpace(rest)
	}

	if name == "" {
		return "", fmt.Errorf("invalid secret reference %q: environment variable name is empty", ref)
	}

	value, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("secret reference %q: environment variable %q is not set", ref, name)
	}
	if value == "" {
		return "", fmt.Errorf("secret reference %q: environment variable %q is empty", ref, name)
	}
	return value, nil
}
