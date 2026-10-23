package step

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	sensitiveQuery = regexp.MustCompile(`(?i)([?&](?:token|access_token|key|signature|x-goog-signature|x-amz-signature)=)[^&\s"'<>]+`)
	bearerToken    = regexp.MustCompile(`(?i)(Bearer\s+)[^\s"',]+`)
)

type redactor struct{ values []string }

func newRedactor(c Config, env, args []string) redactor {
	values := []string{c.FirebaseToken, c.ServiceCredentials}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if key == "FIREBASE_TOKEN" || key == "GOOGLE_APPLICATION_CREDENTIALS" {
			values = append(values, value)
		}
	}
	for i, arg := range args {
		key, value, hasValue := strings.Cut(arg, "=")
		if key == "--token" || key == "--test-password" {
			if !hasValue && i+1 < len(args) {
				value = args[i+1]
			}
			values = append(values, value)
		}
	}
	var nonempty []string
	for _, value := range values {
		if value != "" {
			nonempty = append(nonempty, value, url.QueryEscape(value))
			encoded, _ := json.Marshal(value)
			nonempty = append(nonempty, string(encoded[1:len(encoded)-1]))
		}
	}
	sort.Slice(nonempty, func(i, j int) bool { return len(nonempty[i]) > len(nonempty[j]) })
	return redactor{nonempty}
}

func (r redactor) redact(s string) string {
	for _, value := range r.values {
		s = strings.ReplaceAll(s, value, "[REDACTED]")
	}
	s = sensitiveQuery.ReplaceAllString(s, "${1}[REDACTED]")
	return bearerToken.ReplaceAllString(s, "${1}[REDACTED]")
}
