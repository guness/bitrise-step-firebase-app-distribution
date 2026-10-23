package step

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
)

// Config mirrors the public inputs in step.yml. Bitrise expands environment
// references before invoking the step; input values must never be evaluated again.
type Config struct {
	FirebaseToken      string
	ServiceCredentials string
	AppPath            string
	App                string
	ReleaseNotes       string
	ReleaseNotesLength int
	ReleaseNotesFile   string
	Testers            string
	Groups             string
	Flags              string
	UpgradeFirebase    bool
	Debug              bool
}

func ReadConfig(getenv func(string) string) (Config, error) {
	c := Config{
		FirebaseToken:      getenv("firebase_token"),
		ServiceCredentials: getenv("service_credentials_file"),
		AppPath:            getenv("app_path"),
		App:                getenv("app"),
		ReleaseNotes:       getenv("release_notes"),
		ReleaseNotesFile:   getenv("release_notes_file"),
		Testers:            getenv("testers"),
		Groups:             getenv("groups"),
		Flags:              getenv("flags"),
	}
	var err error
	if raw := getenv("release_notes_length"); raw != "" {
		c.ReleaseNotesLength, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("release_notes_length must be an integer")
		}
	}
	if c.UpgradeFirebase, err = readBool("upgrade_firebase_tools", getenv); err != nil {
		return Config{}, err
	}
	if c.Debug, err = readBool("is_debug", getenv); err != nil {
		return Config{}, err
	}
	return c, nil
}

func readBool(key string, getenv func(string) string) (bool, error) {
	switch getenv(key) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false", key)
	}
}

func normalizeAppPath(path string) (string, error) {
	// Empty Bitrise variables leave pipes around the sole populated path.
	path = strings.Trim(path, "|")
	if path == "" {
		return "", fmt.Errorf("app_path for APK, AAB or IPA is not defined")
	}
	if strings.Contains(path, "|") {
		return "", fmt.Errorf("app_path contains multiple paths; set only one APK, AAB or IPA path")
	}
	if err := regularFile(path); err != nil {
		return "", fmt.Errorf("app_path: %w", err)
	}
	// An absolute positional argument cannot be mistaken for a CLI option, even
	// when the filename starts with a dash.
	return filepath.Abs(path)
}

func regularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	return nil
}

func truncateNotes(notes string, limit int) string {
	if limit <= 0 {
		return notes
	}
	runes := []rune(notes)
	if len(runes) <= limit {
		return notes
	}
	if limit <= 3 {
		return strings.Repeat(".", limit)
	}
	return string(runes[:limit-3]) + "..."
}

func (c Config) arguments() ([]string, error) {
	// Split quotes and whitespace only. No shell, variable expansion, command
	// substitution, redirection or glob expansion is involved (#33, #79).
	extra, err := shellquote.Split(c.Flags)
	if err != nil {
		return nil, fmt.Errorf("invalid quoting in flags: %w", err)
	}
	args := []string{"appdistribution:distribute", c.AppPath, "--app=" + c.App}
	options := []struct{ name, value string }{
		{"token", c.FirebaseToken},
		{"release-notes", truncateNotes(c.ReleaseNotes, c.ReleaseNotesLength)},
		{"release-notes-file", c.ReleaseNotesFile},
		{"testers", c.Testers},
		{"groups", c.Groups},
	}
	for _, option := range options {
		if option.value != "" {
			// The equals form also handles values starting with '-' (#40).
			args = append(args, "--"+option.name+"="+option.value)
		}
	}
	args = append(args, extra...)
	// Debug output supplies the operation JSON, independently of log verbosity.
	return append(args, "--debug"), nil
}
