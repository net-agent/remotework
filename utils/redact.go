package utils

import "regexp"

// sensitiveQueryRe 匹配查询参数中的敏感键值对（不依赖 URL 可解析，兜底更稳）。
// token 同时覆盖 Bearer token 一类的配置/错误文本。
var sensitiveQueryRe = regexp.MustCompile(`(?i)\b(authcode|authcodeRef|auth|authRef|secret|token|password)=[^&\s]*`)

// userinfoRe 匹配 URL 中的 user:password@ 形式
var userinfoRe = regexp.MustCompile(`//([^/@\s:]+):([^/@\s]*)@`)

// RedactText 对任意文本（错误消息、配置片段、日志）应用敏感信息脱敏：
// 查询参数中的 authcode/authcodeRef/auth/authRef/secret/token/password，
// 以及 URL userinfo 中的密码。纯字符串处理，URL 解析失败也能正确脱敏。
func RedactText(s string) string {
	s = userinfoRe.ReplaceAllString(s, "//$1:***@")
	return sensitiveQueryRe.ReplaceAllString(s, "$1=***")
}

// RedactURL 返回抹掉敏感信息后的 URL 字符串，用于日志输出。
// 脱敏规则与 RedactText 一致。
func RedactURL(raw string) string {
	return RedactText(raw)
}
