package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build.aab")
	writeTestFile(t, path, "binary", 0600)
	return Config{AppPath: path, App: "1:1:android:1", FirebaseToken: "stub-token"}
}

func processRunner(t *testing.T, firebaseScript string) (*Runner, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "firebase"), "#!/bin/sh\n"+firebaseScript, 0700)
	writeTestFile(t, filepath.Join(dir, "envman"), "#!/bin/sh\nprintf '%s\\000' \"$@\" >> \"$TEST_EXPORTS\"\n", 0700)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGUMENTS", filepath.Join(dir, "arguments"))
	t.Setenv("TEST_EXPORTS", filepath.Join(dir, "exports"))
	t.Setenv("TEST_FIREBASE_OUTPUT", filepath.Join(dir, "output"))
	writeTestFile(t, filepath.Join(dir, "output"), fixture(t, "firebase-stub-full-debug.sh"), 0600)
	log := &bytes.Buffer{}
	r := NewRunner(log)
	r.tempDir = t.TempDir()
	return r, log, dir
}

func readArguments(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestRealProcessPreservesInputsAndExports(t *testing.T) {
	r, log, dir := processRunner(t, "printf '%s\\000' \"$@\" > \"$TEST_ARGUMENTS\"\ncat \"$TEST_FIREBASE_OUTPUT\"\n")
	c := testConfig(t)
	marker := filepath.Join(dir, "SHOULD_NOT_EXIST")
	c.ReleaseNotes = "- `touch " + marker + "` $(touch " + marker + ")\nRevert \"quotes\" 'single' $HOME \\ backslash 🚀\n\n"
	c.Testers = `a@example.com,$HOME,"literal"`
	c.Groups = `qa;echo injected`
	c.FirebaseToken = `token$with"quotes` + "`backticks`"
	c.App = `1:1:android:$literal`
	c.ReleaseNotesFile = filepath.Join(dir, `notes 'quoted' $literal.txt`)
	writeTestFile(t, c.ReleaseNotesFile, "file notes", 0600)
	c.Flags = `--test-devices-file "./test devices.txt" --test-password '` + "`touch " + marker + "` $(touch " + marker + ")" + `' --test-non-blocking`
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	args := readArguments(t, filepath.Join(dir, "arguments"))
	want := []string{"appdistribution:distribute", c.AppPath, "--app=" + c.App,
		"--token=" + c.FirebaseToken, "--release-notes=" + c.ReleaseNotes,
		"--release-notes-file=" + c.ReleaseNotesFile, "--testers=" + c.Testers, "--groups=" + c.Groups,
		"--test-devices-file", "./test devices.txt", "--test-password", "`touch " + marker + "` $(touch " + marker + ")", "--test-non-blocking", "--debug"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("argv changed input bytes:\ngot  %#v\nwant %#v", args, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("input text was executed: %v", err)
	}
	exports := readArguments(t, filepath.Join(dir, "exports"))
	got := map[string]string{}
	for i := 0; i < len(exports); i += 5 {
		if i+5 > len(exports) || exports[i] != "add" || exports[i+1] != "--key" || exports[i+3] != "--value" {
			t.Fatalf("invalid envman invocation: %#v", exports)
		}
		got[exports[i+2]] = exports[i+4]
	}
	if len(got) != 5 || got["FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID"] != "PRIMARYID" || got["FIREBASE_APP_DISTRIBUTION_RELEASE_ID"] != "PRIMARYID" {
		t.Fatalf("release ID compatibility outputs = %#v", got)
	}
	if !strings.Contains(got["FIREBASE_BITRISE_BINARY_DOWNLOAD_URL"], "&token=FAKETOKEN") {
		t.Fatal("redaction must not change exported URLs")
	}
	if strings.Contains(log.String(), "FAKETOKEN") || strings.Contains(log.String(), c.FirebaseToken) || strings.Contains(log.String(), "[apiv2][body]") {
		t.Fatalf("ordinary logs leaked secrets or debug output: %s", log)
	}
	if !strings.Contains(log.String(), "Success") {
		t.Fatal("missing success message")
	}
}

func TestFirebaseFailurePrintsBothStreamsAndExitCode(t *testing.T) {
	r, log, dir := processRunner(t, "echo 'unprefixed stdout diagnostic'\necho 'Error: expired stub-token' >&2\nexit 42\n")
	err := r.Run(context.Background(), testConfig(t))
	if err == nil || ExitCode(err) != 42 {
		t.Fatalf("error = %v, exit = %d", err, ExitCode(err))
	}
	for _, want := range []string{"unprefixed stdout diagnostic", "Error: expired [REDACTED]"} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("missing diagnostic %q: %s", want, log)
		}
	}
	if strings.Contains(log.String(), "Success") || strings.Contains(log.String(), "stub-token") {
		t.Fatalf("incorrect failure logs: %s", log)
	}
	if _, err := os.Stat(filepath.Join(dir, "exports")); !os.IsNotExist(err) {
		t.Fatal("outputs exported after a failed Firebase command")
	}
}

func TestExportFailure(t *testing.T) {
	r, log, dir := processRunner(t, "cat \"$TEST_FIREBASE_OUTPUT\"\n")
	writeTestFile(t, filepath.Join(dir, "envman"), "#!/bin/sh\necho 'could not write env store' >&2\nexit 9\n", 0700)
	err := r.Run(context.Background(), testConfig(t))
	if err == nil || ExitCode(err) != 9 || !strings.Contains(err.Error(), "export FIREBASE_BITRISE_CONSOLE_URL") || !strings.Contains(log.String(), "could not write env store") || strings.Contains(log.String(), "Success") {
		t.Fatalf("error = %v; logs = %s", err, log)
	}
}

func TestMissingOutputsAreExportedEmpty(t *testing.T) {
	r, log, dir := processRunner(t, "echo '✔ uploaded successfully'\n")
	if err := r.Run(context.Background(), testConfig(t)); err != nil {
		t.Fatal(err)
	}
	args := readArguments(t, filepath.Join(dir, "exports"))
	if len(args) != 25 {
		t.Fatalf("got %d export arguments", len(args))
	}
	for i := 4; i < len(args); i += 5 {
		if args[i] != "" {
			t.Errorf("missing output was not exported empty: %q", args[i])
		}
	}
	if strings.Count(log.String(), "could not be extracted") != 5 {
		t.Fatalf("missing field warnings absent: %s", log)
	}
}

func TestInputValidationBeforeExternalWork(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"app", func(c *Config) { c.App = "" }},
		{"auth", func(c *Config) { c.FirebaseToken = "" }},
		{"app path", func(c *Config) { c.AppPath += "|other.apk" }},
		{"flags", func(c *Config) { c.Flags = `--groups "bad` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner(&bytes.Buffer{})
			r.lookPath = func(string) (string, error) { t.Fatal("validation performed external work"); return "", nil }
			c := testConfig(t)
			tc.edit(&c)
			if err := r.Run(context.Background(), c); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestMissingReleaseNotesFileRemainsOptional(t *testing.T) {
	r, log, dir := processRunner(t, "printf '%s\\000' \"$@\" > \"$TEST_ARGUMENTS\"\ncat \"$TEST_FIREBASE_OUTPUT\"\n")
	c := testConfig(t)
	c.ReleaseNotesFile = filepath.Join(dir, "missing.txt")
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(readArguments(t, filepath.Join(dir, "arguments")), "\n"), "--release-notes-file") || !strings.Contains(log.String(), "skipping this option") {
		t.Fatal("missing notes file should warn and omit the option")
	}
}

func TestDownloadedCredentialsArePrivateScopedAndCleaned(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("uploadFails=%t", fail), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.RawQuery != "signature=secret" {
					t.Error("signed credential URL changed")
				}
				fmt.Fprint(w, `{"type":"service_account","private_key":"fake"}`)
			}))
			defer server.Close()
			log := &bytes.Buffer{}
			r := NewRunner(log)
			r.tempDir = t.TempDir()
			r.env = []string{"FIREBASE_TOKEN=stale-secret", "GOOGLE_APPLICATION_CREDENTIALS=old.json", "UNCHANGED=ok"}
			r.lookPath = func(name string) (string, error) { return name, nil }
			var credentials string
			r.command = func(_ context.Context, name string, args, env []string) ([]byte, error) {
				if name == "envman" {
					return nil, nil
				}
				for _, e := range env {
					key, value, _ := strings.Cut(e, "=")
					if key == "FIREBASE_TOKEN" {
						t.Fatal("inherited token overrides explicit service credentials")
					}
					if key == "GOOGLE_APPLICATION_CREDENTIALS" {
						credentials = value
					}
				}
				info, err := os.Stat(credentials)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("credential permissions: %v, %v", info, err)
				}
				data, err := os.ReadFile(credentials)
				if err != nil || !strings.Contains(string(data), "service_account") {
					t.Fatalf("credentials unavailable to subprocess: %v", err)
				}
				if fail {
					return []byte("Error: auth failed"), errors.New("upload failed")
				}
				return []byte(fixture(t, "firebase-stub-full-debug.sh") + "[debug] credentials: " + credentials), nil
			}
			c := testConfig(t)
			c.FirebaseToken = ""
			c.Debug = true
			c.ServiceCredentials = server.URL + "/service.json?signature=secret"
			err := r.Run(context.Background(), c)
			if (err != nil) != fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if credentials == "" {
				t.Fatal("upload was not invoked")
			}
			if _, err := os.Stat(credentials); !os.IsNotExist(err) {
				t.Fatal("downloaded credentials not cleaned up")
			}
			if !reflect.DeepEqual(r.env, []string{"FIREBASE_TOKEN=stale-secret", "GOOGLE_APPLICATION_CREDENTIALS=old.json", "UNCHANGED=ok"}) {
				t.Fatal("parent environment mutated")
			}
			if strings.Contains(log.String(), "stale-secret") || strings.Contains(log.String(), server.URL) || strings.Contains(log.String(), credentials) {
				t.Fatal("credential values logged")
			}
		})
	}
}

func TestLocalCredentialsAndTokenPrecedence(t *testing.T) {
	r := NewRunner(&bytes.Buffer{})
	r.env = []string{"FIREBASE_TOKEN=stale"}
	path := filepath.Join(t.TempDir(), "service account.json")
	writeTestFile(t, path, "{}", 0600)
	c := Config{ServiceCredentials: path}
	env, cleanup, err := r.credentials(context.Background(), c, newRedactor(c, r.env, nil))
	if err != nil || !reflect.DeepEqual(env, []string{"GOOGLE_APPLICATION_CREDENTIALS=" + path}) {
		t.Fatalf("credential env: %#v, %v", env, err)
	}
	cleanup()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("caller-owned credentials were removed")
	}
	c.FirebaseToken = "explicit"
	c.ServiceCredentials = "not-a-real-file"
	env, cleanup, err = r.credentials(context.Background(), c, newRedactor(c, r.env, nil))
	if err != nil || !reflect.DeepEqual(env, r.env) {
		t.Fatalf("explicit token should take precedence: %#v, %v", env, err)
	}
	cleanup()
}

func TestDownloadRejectsHTTPFailuresEmptyAndOversizedBodies(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"forbidden", "denied", http.StatusForbidden},
		{"empty", "", http.StatusOK},
		{"oversized", "too many bytes", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			r := NewRunner(&bytes.Buffer{})
			r.tempDir = t.TempDir()
			if _, err := r.download(context.Background(), server.URL, "download-*", 5); err == nil {
				t.Fatal("invalid download accepted")
			}
			files, err := os.ReadDir(r.tempDir)
			if err != nil || len(files) != 0 {
				t.Fatalf("partial downloads left behind: %v, %v", files, err)
			}
		})
	}
}

func TestInstallerAndUpgrade(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(fmt.Sprintf("upgrade=%t", upgrade), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "fake installer") }))
			defer server.Close()
			r := NewRunner(&bytes.Buffer{})
			r.tempDir = t.TempDir()
			r.installerURL = server.URL
			installed := upgrade // An existing installation must still be upgraded.
			r.lookPath = func(name string) (string, error) {
				if installed {
					return "/bin/firebase", nil
				}
				return "", exec.ErrNotFound
			}
			var installer string
			var calls []string
			r.command = func(_ context.Context, name string, args, env []string) ([]byte, error) {
				calls = append(calls, name)
				if name == "bash" {
					installer = args[0]
					data, err := os.ReadFile(installer)
					if err != nil || string(data) != "fake installer" {
						t.Fatalf("installer not downloaded: %v", err)
					}
					if strings.Contains(strings.Join(env, "\n"), "upgrade=true") != upgrade {
						t.Fatalf("upgrade environment: %v", env)
					}
					installed = true
					return []byte("installed"), nil
				}
				if !reflect.DeepEqual(args, []string{"--version"}) {
					t.Fatalf("installation not checked: %v", args)
				}
				return []byte("14.0.0"), nil
			}
			path, err := r.firebase(context.Background(), upgrade, []string{"upgrade=true"}, redactor{})
			if err != nil || path != "/bin/firebase" || !reflect.DeepEqual(calls, []string{"bash", "/bin/firebase"}) {
				t.Fatalf("installer calls: %v, path: %q, err: %v", calls, path, err)
			}
			if _, err := os.Stat(installer); !os.IsNotExist(err) {
				t.Fatal("installer file not removed")
			}
		})
	}
}

func TestInstallerFailureAndMissingExecutable(t *testing.T) {
	for _, failure := range []string{"download", "install", "missing", "version", "empty-version"} {
		t.Run(failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if failure == "download" {
					w.WriteHeader(http.StatusNotFound)
				}
				fmt.Fprint(w, "installer")
			}))
			defer server.Close()
			r := NewRunner(&bytes.Buffer{})
			r.tempDir = t.TempDir()
			r.installerURL = server.URL
			r.lookPath = func(string) (string, error) {
				if failure == "missing" {
					return "", exec.ErrNotFound
				}
				return "firebase", nil
			}
			r.command = func(_ context.Context, name string, args, env []string) ([]byte, error) {
				if name == "bash" && failure == "install" || name == "firebase" && failure == "version" {
					return []byte("diagnostic"), errors.New("failed")
				}
				return nil, nil
			}
			if _, err := r.firebase(context.Background(), true, nil, redactor{}); err == nil {
				t.Fatal("installation failure ignored")
			}
			files, err := os.ReadDir(r.tempDir)
			if err != nil || len(files) != 0 {
				t.Fatalf("installer not cleaned: %v, %v", files, err)
			}
		})
	}
}

func TestCanceledCommand(t *testing.T) {
	r, _, _ := processRunner(t, "exit 0\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := r.Run(ctx, testConfig(t))
	if !errors.Is(err, context.Canceled) || ExitCode(err) != 130 {
		t.Fatalf("cancellation not preserved: %v", err)
	}
}
