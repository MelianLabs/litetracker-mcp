package notify

import (
	"encoding/json"
	"os/exec"
	"strings"
)

func Send(title, message string) {
	// Use JSON encoding for safe string escaping, then extract the quoted content.
	// This avoids AppleScript injection via crafted title/message strings.
	safeTitle := escapeForAppleScript(title)
	safeMessage := escapeForAppleScript(message)
	script := `display notification ` + safeMessage + ` with title ` + safeTitle
	_ = exec.Command("osascript", "-e", script).Start()
}

func escapeForAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	// json.Marshal produces a properly escaped JSON string with surrounding quotes.
	// AppleScript string literals use the same double-quote + backslash escaping.
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
