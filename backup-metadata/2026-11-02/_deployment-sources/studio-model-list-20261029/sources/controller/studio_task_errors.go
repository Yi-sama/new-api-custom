package controller

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var studioErrorCredentialPattern = regexp.MustCompile(`(?i)(bearer\s+|(?:api[_-]?key|authorization|token|password)\s*[:=]\s*["']?)[^\s"',;]+|sk-[a-zA-Z0-9_-]{8,}`)

func studioTaskErrorText(message string) string {
	message = studioErrorCredentialPattern.ReplaceAllString(message, "${1}[redacted]")
	message = strings.Join(strings.Fields(common.MaskSensitiveInfo(message)), " ")
	text := []rune(message)
	if len(text) > 1000 {
		message = string(text[:1000]) + "..."
	}
	return message
}

func studioTaskFailureMessage(status int, code, message, jobID string) string {
	message = studioTaskErrorText(message)
	if message == "" {
		message = "generation request was rejected"
	}
	return fmt.Sprintf("%s (HTTP %d; code: %s; reference: %s)", message, status, studioTaskErrorText(code), jobID)
}
