package step

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "_fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	// Read the exact logged content from the existing Bash stub's heredoc.
	_, dataString, found := strings.Cut(string(data), "cat <<'STUB_OUT'\n")
	if !found {
		t.Fatal("fixture heredoc not found")
	}
	dataString, _, _ = strings.Cut(dataString, "\nSTUB_OUT")
	return dataString + "\n"
}

func TestParseExistingFixtures(t *testing.T) {
	for _, tc := range []struct{ file, id, binary string }{
		{"firebase-stub-full-debug.sh", "PRIMARYID", "BINID"},
		{"firebase-stub-labeled-only.sh", "FALLBACKID", "BIN2"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			o := parseOutput(fixture(t, tc.file))
			if o.ReleaseID != tc.id || !strings.Contains(o.ConsoleURL, "/releases/"+tc.id+"?") || !strings.Contains(o.TestingURL, "/releases/"+tc.id+"?") || !strings.Contains(o.BinaryDownloadURL, "/binaries/"+tc.binary+"/") || !strings.Contains(o.BinaryDownloadURL, "&token=") {
				t.Fatalf("incorrect output: %#v", o)
			}
			for _, variable := range o.variables() {
				if variable.value == "" {
					t.Errorf("empty output %s", variable.key)
				}
			}
		})
	}
}

func TestParseOutputFallbackAndPrecedence(t *testing.T) {
	fallback := fixture(t, "firebase-stub-labeled-only.sh")
	for _, tc := range []struct{ name, prefix, id, console string }{
		{"whitespace JSON", `[apiv2][body] GET url {"done": true, "response": {"release": {"name":"projects/p/apps/a/releases/JSON-id_1", "firebaseConsoleUri":"https://console.example/releases/JSON-id_1"}}}`, "JSON-id_1", "https://console.example/releases/JSON-id_1"},
		{"malformed JSON", `[apiv2][body] GET url {"done":true,"response":{"release":`, "FALLBACKID", ""},
		{"pending operation", `[apiv2][body] GET url {"done":false,"response":{"release":{"name":"projects/p/apps/a/releases/WRONG"}}}`, "FALLBACKID", ""},
		{"unrelated operation", `[apiv2][body] GET url {"done":true,"response":{"name":"WRONG"}}`, "FALLBACKID", ""},
		{"null release", `[apiv2][body] GET url {"done":true,"response":{"release":null}}`, "FALLBACKID", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := parseOutput(tc.prefix + "\n" + fallback)
			if o.ReleaseID != tc.id || (tc.console != "" && o.ConsoleURL != tc.console) || o.TestingURL == "" || o.BinaryDownloadURL == "" {
				t.Fatalf("parsed %#v", o)
			}
		})
	}
	// A later valid completed release wins; a later unrelated JSON body does not.
	first := `[apiv2][body] GET url {"done":true,"response":{"release":{"name":"projects/p/apps/a/releases/OLD"}}}`
	last := `[apiv2][body] GET url {"done":true,"response":{"release":{"name":"projects/p/apps/a/releases/NEW"}}}`
	if got := parseOutput(first + "\n" + last + "\n[apiv2][body] {}\n"); got.ReleaseID != "NEW" {
		t.Fatalf("wrong final operation: %#v", got)
	}
	if got := parseOutput("https://www.googleapis.com/auth/firebase\nhttps://example.com/releases/WRONG"); got != (outputs{}) {
		t.Fatalf("unrelated URLs became outputs: %#v", got)
	}
}

func TestParseLargeAndColoredOutput(t *testing.T) {
	data := "\x1b[32m✔ View this release in the Firebase console: https://example.com/releases/a-b_c?x=1&y=2\x1b[0m\r\n"
	if got := parseOutput(data); got.ReleaseID != "a-b_c" || got.ConsoleURL != "https://example.com/releases/a-b_c?x=1&y=2" {
		t.Fatalf("colored output = %#v", got)
	}
	data = `[apiv2][body] {"done":true,"response":{"release":{"name":"projects/p/apps/a/releases/LARGE","notes":"` + strings.Repeat("x", 128*1024) + `"}}}`
	if got := parseOutput(data); got.ReleaseID != "LARGE" {
		t.Fatalf("large JSON body was lost: %#v", got)
	}
}

func TestDisplayOutput(t *testing.T) {
	raw := "[debug] metadata\n\x1b[32m✔ success\x1b[0m\ni upload\n! warn\n⚠ warning\nError: failure\nplain diagnostic\n"
	for _, tc := range []struct{ debug, failed bool }{{true, false}, {false, true}, {true, true}} {
		if got := displayOutput(raw, tc.debug, tc.failed); got != raw {
			t.Fatalf("debug/failure diagnostics dropped: %q", got)
		}
	}
	got := displayOutput(raw, false, false)
	if strings.Contains(got, "metadata") || strings.Contains(got, "plain diagnostic") || !strings.Contains(got, "Error: failure") || !strings.Contains(got, "✔ success") {
		t.Fatalf("normal display = %q", got)
	}
}
