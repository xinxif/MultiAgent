// 敏感数据脱敏：日志与消息存储前对 PII 做掩码。
package security

import (
	"regexp"
	"strings"
)
var (
	phoneRe   = regexp.MustCompile(`1[3-9]\d{9}`)
	emailRe   = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	idCardRe  = regexp.MustCompile(`\d{17}[\dXx]|\d{15}`)
)

// MaskPII 对文本中的手机号/邮箱/身份证做掩码。
// 手机号 138****1234；邮箱 a***b@x.com 保留首尾；身份证仅保留前 4 位。
func MaskPII(s string) string {
	s = phoneRe.ReplaceAllStringFunc(s, func(m string) string {
		return m[:3] + "****" + m[7:]
	})
	s = emailRe.ReplaceAllStringFunc(s, func(m string) string {
		at := len(m) - len(m[strings.LastIndexByte(m, '@'):])
		return m[0:1] + "***" + m[at-1:]
	})
	s = idCardRe.ReplaceAllStringFunc(s, func(m string) string {
		return m[:4] + "***********" + m[len(m)-1:]
	})
	return s
}
