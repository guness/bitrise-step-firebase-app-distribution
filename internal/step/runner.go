package step

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner owns process, network, and environment boundaries. Its defaults use the
// real Firebase CLI and envman; tests replace only those external boundaries.
type Runner struct {
	log          io.Writer
	env          []string
	client       *http.Client
	command      func(context.Context, string, []string, []string) ([]byte, error)
	lookPath     func(string) (string, error)
	installerURL string
	tempDir      string
}

func NewRunner(log io.Writer) *Runner {
	return &Runner{
		log: log,
		env: os.Environ(),
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
		command: func(ctx context.Context, name string, args, env []string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = env
			return cmd.CombinedOutput()
		},
		lookPath:     exec.LookPath,
		installerURL: "https://firebase.tools",
	}
}

func (r *Runner) Run(ctx context.Context, c Config) (err error) {
	redact := newRedactor(c, r.env, nil)
	// Sanitize the final error too, while preserving its underlying exit status.
	defer func() {
		if err != nil {
			err = safeError{redact.redact(err.Error()), err}
		}
	}()
	if c.App == "" {
		return fmt.Errorf("Firebase App ID (app) is not defined")
	}
	if c.FirebaseToken == "" && c.ServiceCredentials == "" {
		return fmt.Errorf("no authentication input was defined; set firebase_token or service_credentials_file")
	}
	if c.AppPath, err = normalizeAppPath(c.AppPath); err != nil {
		return err
	}
	if c.ReleaseNotesFile != "" {
		if err := regularFile(c.ReleaseNotesFile); err != nil {
			r.print(redact, "Warning: release_notes_file does not exist or is not a regular file; skipping this option")
			c.ReleaseNotesFile = ""
		}
	}
	args, err := c.arguments()
	if err != nil {
		return err
	}
	redact = newRedactor(c, r.env, args)
	envman, err := r.lookPath("envman")
	if err != nil {
		return fmt.Errorf("envman is required to export Bitrise outputs: %w", err)
	}
	env, cleanup, err := r.credentials(ctx, c, redact)
	if err != nil {
		return err
	}
	defer cleanup()
	redact = newRedactor(c, append(append([]string(nil), r.env...), env...), args)
	firebase, err := r.firebase(ctx, c.UpgradeFirebase, env, redact)
	if err != nil {
		return err
	}
	r.print(redact, "Deploying build to Firebase")
	raw, commandErr := r.command(ctx, firebase, args, env)
	r.print(redact, displayOutput(string(raw), c.Debug, commandErr != nil))
	if commandErr != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("Firebase distribution interrupted: %w", ctx.Err())
		}
		return fmt.Errorf("Firebase distribution failed: %w", commandErr)
	}
	for _, output := range parseOutput(string(raw)).variables() {
		if output.value == "" {
			r.print(redact, "Warning: "+output.key+" could not be extracted from Firebase output")
		}
		exportLog, err := r.command(ctx, envman, []string{"add", "--key", output.key, "--value", output.value}, r.env)
		if err != nil {
			r.print(redact, string(exportLog))
			return fmt.Errorf("export %s: %w", output.key, err)
		}
	}
	r.print(redact, "Success")
	return nil
}

func (r *Runner) credentials(ctx context.Context, c Config, redact redactor) ([]string, func(), error) {
	env := append([]string(nil), r.env...)
	cleanup := func() {}
	if c.FirebaseToken != "" {
		if c.ServiceCredentials != "" {
			r.print(redact, "Warning: both authentication inputs are defined; using firebase_token")
		}
		return env, cleanup, nil
	}
	path := c.ServiceCredentials
	if u, err := url.Parse(path); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		r.print(redact, "Downloading service credentials")
		path, err = r.download(ctx, path, "firebase-credentials-*.json", 10<<20)
		if err != nil {
			return nil, cleanup, fmt.Errorf("download service credentials: %w", err)
		}
		cleanup = func() { _ = os.Remove(path) }
	} else if err := regularFile(path); err != nil {
		return nil, cleanup, fmt.Errorf("service_credentials_file: %w", err)
	}
	// An unrelated FIREBASE_TOKEN in the build environment must not override an
	// explicitly selected service account (#43). Only the child env is changed.
	for _, item := range env {
		if key, value, _ := strings.Cut(item, "="); key == "FIREBASE_TOKEN" && value != "" {
			r.print(redact, "Warning: ignoring inherited FIREBASE_TOKEN in favor of service_credentials_file")
			break
		}
	}
	env = withoutEnv(env, "FIREBASE_TOKEN")
	env = append(withoutEnv(env, "GOOGLE_APPLICATION_CREDENTIALS"), "GOOGLE_APPLICATION_CREDENTIALS="+path)
	return env, cleanup, nil
}

func (r *Runner) firebase(ctx context.Context, upgrade bool, env []string, redact redactor) (string, error) {
	if path, err := r.lookPath("firebase"); err == nil && !upgrade {
		r.print(redact, "Firebase CLI is already installed. Skipping installation.")
		return path, nil
	}
	r.print(redact, "Installing Firebase CLI")
	installer, err := r.download(ctx, r.installerURL, "firebase-installer-*.sh", 2<<20)
	if err != nil {
		return "", fmt.Errorf("download Firebase installer: %w", err)
	}
	defer os.Remove(installer)
	installEnv := withoutEnv(env, "upgrade")
	if upgrade {
		installEnv = append(installEnv, "upgrade=true")
	}
	output, err := r.command(ctx, "bash", []string{installer}, installEnv)
	r.print(redact, string(output))
	if err != nil {
		return "", fmt.Errorf("install Firebase CLI: %w", err)
	}
	path, err := r.lookPath("firebase")
	if err != nil {
		return "", fmt.Errorf("Firebase installer finished but firebase is not on PATH: %w", err)
	}
	version, err := r.command(ctx, path, []string{"--version"}, env)
	if err != nil {
		r.print(redact, string(version))
		return "", fmt.Errorf("installed Firebase CLI is not executable: %w", err)
	}
	if strings.TrimSpace(string(version)) == "" {
		return "", fmt.Errorf("installed Firebase CLI returned an empty version")
	}
	return path, nil
}

func (r *Runner) download(ctx context.Context, source, pattern string, maxBytes int64) (path string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", err
	}
	response, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %s", response.Status)
	}
	file, err := os.CreateTemp(r.tempDir, pattern)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
		if err != nil {
			_ = os.Remove(file.Name())
		}
	}()
	n, err := io.Copy(file, io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxBytes {
		return "", fmt.Errorf("download exceeded %d bytes", maxBytes)
	}
	if n == 0 {
		return "", fmt.Errorf("download was empty")
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func (r *Runner) print(redact redactor, message string) {
	if message != "" {
		fmt.Fprintln(r.log, redact.redact(strings.TrimRight(message, "\n")))
	}
}

func withoutEnv(env []string, key string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if name, _, _ := strings.Cut(entry, "="); name != key {
			result = append(result, entry)
		}
	}
	return result
}

type safeError struct {
	message string
	cause   error
}

func (e safeError) Error() string { return e.message }
func (e safeError) Unwrap() error { return e.cause }

func ExitCode(err error) int {
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}
