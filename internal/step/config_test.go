package step

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReadConfig(t *testing.T) {
	inputs := map[string]string{
		"firebase_token": "token", "service_credentials_file": "service.json",
		"app_path": "app.aab", "app": "1:2:android:3", "release_notes": "notes\n",
		"release_notes_length": "20", "release_notes_file": "notes.txt",
		"testers": "a@example.com,b@example.com", "groups": "qa,dev",
		"flags": `--groups-file "QA groups.txt"`, "upgrade_firebase_tools": "true", "is_debug": "true",
	}
	c, err := ReadConfig(func(key string) string { return inputs[key] })
	if err != nil {
		t.Fatal(err)
	}
	want := Config{"token", "service.json", "app.aab", "1:2:android:3", "notes\n", 20, "notes.txt", "a@example.com,b@example.com", "qa,dev", inputs["flags"], true, true}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("config = %#v, want %#v", c, want)
	}
	defaults, err := ReadConfig(func(string) string { return "" })
	if err != nil || defaults != (Config{}) {
		t.Fatalf("blank optional values = %#v, %v", defaults, err)
	}
	for _, key := range []string{"release_notes_length", "upgrade_firebase_tools", "is_debug"} {
		t.Run(key, func(t *testing.T) {
			_, err := ReadConfig(func(k string) string {
				if k == key {
					return "invalid"
				}
				return ""
			})
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected a validation error for %s, got %v", key, err)
			}
		})
	}
}

func TestNormalizeAppPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build with 'quotes' and $dollars.aab")
	if err := os.WriteFile(path, []byte("binary"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{path, path + "||", "|" + path + "|", "||" + path, path + "|", "|" + path} {
		got, err := normalizeAppPath(value)
		if err != nil || got != path {
			t.Errorf("normalize %q = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "||", path + "|" + path, "||" + path + "|another", path + ".missing", filepath.Dir(path)} {
		if got, err := normalizeAppPath(value); err == nil {
			t.Errorf("normalize %q unexpectedly succeeded: %q", value, got)
		}
	}
}

func TestTruncateNotes(t *testing.T) {
	for _, tc := range []struct {
		notes string
		limit int
		want  string
	}{
		{"abcdef", 0, "abcdef"}, {"abcdef", -1, "abcdef"}, {"abcdef", 6, "abcdef"},
		{"abcdef", 5, "ab..."}, {"abcdef", 3, "..."}, {"abcdef", 2, ".."}, {"abcdef", 1, "."},
		{"öğ🚀世界xyz", 6, "öğ🚀..."}, {"notes\n\n", 20, "notes\n\n"}, {"", 1, ""},
	} {
		got := truncateNotes(tc.notes, tc.limit)
		if got != tc.want || !utf8.ValidString(got) {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.notes, tc.limit, got, tc.want)
		}
	}
}

func TestArgumentsPreserveLiteralValues(t *testing.T) {
	// These are the failure cases from GitHub #17, #33, #40 and #79.
	for _, notes := range []string{
		"- `androidMaterial` = '1.2.1'",
		`Merge pull request. Revert "XXXXXX"`,
		"- Add feature 1\n- Add feature 2\n",
		"$HOME $(touch /tmp/never-run) `uname` \\ emoji 🚀\n\n",
		`"; echo injected; #`,
	} {
		c := Config{AppPath: "/app.apk", App: "1:2:android:3", ReleaseNotes: notes}
		args, err := c.arguments()
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != 5 || args[3] != "--release-notes="+notes || args[4] != "--debug" {
			t.Fatalf("notes lost their argument boundary: %#v", args)
		}
	}
}

func TestAdditionalFlags(t *testing.T) {
	for _, tc := range []struct {
		flags string
		want  []string
	}{
		{`--test-devices-file "./test devices.txt" --test-non-blocking`, []string{"--test-devices-file", "./test devices.txt", "--test-non-blocking"}},
		{`--testers-file='a file.txt' --groups-file=groups.txt`, []string{"--testers-file=a file.txt", "--groups-file=groups.txt"}},
		{"--groups-file path\\ with\\ spaces\n--test-password ''", []string{"--groups-file", "path with spaces", "--test-password", ""}},
		{"--test-password '`whoami` $(id) $HOME ; & *'", []string{"--test-password", "`whoami` $(id) $HOME ; & *"}},
	} {
		args, err := (Config{AppPath: "/app.apk", App: "app", Flags: tc.flags}).arguments()
		if err != nil {
			t.Fatal(err)
		}
		if got := args[3 : len(args)-1]; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("flags %q = %#v, want %#v", tc.flags, got, tc.want)
		}
	}
	for _, flags := range []string{`--groups "unterminated`, `--testers 'unterminated`, "--groups trailing\\"} {
		if _, err := (Config{Flags: flags}).arguments(); err == nil {
			t.Errorf("malformed flags accepted: %q", flags)
		}
	}
}
