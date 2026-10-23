package step

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	c := Config{FirebaseToken: "input-secret", ServiceCredentials: "https://storage.example/key.json?signature=credential-secret"}
	r := newRedactor(c, []string{"FIREBASE_TOKEN=env-secret", "GOOGLE_APPLICATION_CREDENTIALS=/private/key.json"}, []string{"--token=flag-secret", "--test-password", "test-secret"})
	raw := c.ServiceCredentials + " input-secret env-secret flag-secret test-secret /private/key.json\n" +
		"https://example.com/app.aab?x=1&token=download-secret&y=2\nAuthorization: Bearer bearer-secret"
	got := r.redact(raw)
	for _, secret := range []string{"input-secret", "env-secret", "flag-secret", "test-secret", "credential-secret", "/private/key.json", "download-secret", "bearer-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("secret %q remains in logs: %s", secret, got)
		}
	}
	if !strings.Contains(got, "?x=1&token=[REDACTED]&y=2") {
		t.Errorf("redaction destroyed surrounding diagnostic context: %s", got)
	}
}

func TestRedactionOfJSONEncodedPassword(t *testing.T) {
	password := "password with \"quotes\" and\nnewline"
	r := newRedactor(Config{}, nil, []string{"--test-password=" + password})
	encoded, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.redact(string(encoded)); strings.Contains(got, "password with") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("JSON diagnostic leaked password: %s", got)
	}
}
