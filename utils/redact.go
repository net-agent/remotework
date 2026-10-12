package utils

import "regexp"

// sensitiveQueryRe 匹配查询参数中的敏感键值对（不依赖 URL 可解析，兜底更稳）
var sensitiveQueryRe = regexp.MustCompile(`(?i)\b(authcode|authcodeRef|auth|authRef|secret|password)=[^&\s]*`)

// userinfoRe 匹配 URL 中的 user:password@ 形式
var userinfoRe = regexp.MustCompile(`//([^/@\s:]+):([^/@\s]*)@`)

// RedactURL 返回抹掉敏感信息后的 URL 字符串，用于日志输出。
// 脱敏范围：查询参数中的 authcode/authcodeRef/auth/authRef/secret/password，
// 以及 userinfo 中的密码。纯字符串处理，URL 解析失败也能正确脱敏。
func RedactURL(raw string) string {
	raw = userinfoRe.ReplaceAllString(raw, "//$1:***@")
	return sensitiveQueryRe.ReplaceAllString(raw, "$1=***")
}
