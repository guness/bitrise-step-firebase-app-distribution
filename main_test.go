package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the actual entrypoint in a subprocess so its exit code is exercised too.
func TestMain(m *testing.M) {
	if os.Getenv("FIREBASE_STEP_TEST_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func entrypointEnv(t *testing.T, dir, firebaseScript string) []string {
	t.Helper()
	write := func(name, content string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("firebase", "#!/bin/sh\n"+firebaseScript, 0700)
	write("envman", "#!/bin/sh\nprintf '%s\\000' \"$@\" >> \"$ENTRYPOINT_EXPORTS\"\n", 0700)
	write("app.aab", "binary", 0600)
	var env []string
	// Remove step inputs inherited from any surrounding Bitrise workflow.
	inputs := "|firebase_token|service_credentials_file|app_path|app|release_notes|release_notes_length|release_notes_file|testers|groups|flags|upgrade_firebase_tools|is_debug|PATH|FIREBASE_STEP_TEST_MAIN|"
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if !strings.Contains(inputs, "|"+name+"|") {
			env = append(env, item)
		}
	}
	return append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FIREBASE_STEP_TEST_MAIN=1", "app_path=app.aab", "app=1:1:android:1", "firebase_token=test-token",
		"ENTRYPOINT_EXPORTS="+filepath.Join(dir, "exports"))
}

func TestEntrypointExitStatusAndDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, script, wantLog string
		code                  int
	}{
		{"success", "echo '✔ uploaded successfully'\n", "Success", 0},
		{"firebase failure", "echo 'plain failure detail' >&2\nexit 42\n", "plain failure detail", 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0])
			cmd.Dir = dir
			cmd.Env = entrypointEnv(t, dir, tc.script)
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState.ExitCode() != tc.code || !strings.Contains(string(output), tc.wantLog) {
				t.Fatalf("exit = %d, error = %v, output = %s", cmd.ProcessState.ExitCode(), err, output)
			}
			if tc.code == 0 {
				data, err := os.ReadFile(filepath.Join(dir, "exports"))
				if err != nil || strings.Count(string(data), "--key\x00") != 5 {
					t.Fatalf("entrypoint failed to export all outputs: %v, %q", err, data)
				}
			}
		})
	}
}

func TestShellLauncherPreservesCallerWorkingDirectory(t *testing.T) {
	script, err := filepath.Abs("step.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command(script)
	cmd.Dir = dir
	cmd.Env = entrypointEnv(t, dir, "echo '✔ uploaded successfully'\n")
	// Keep build products isolated and verify that the launcher removes them.
	buildTemp := t.TempDir()
	var env []string
	for _, entry := range cmd.Env {
		if !strings.HasPrefix(entry, "TMPDIR=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "TMPDIR="+buildTemp)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "Success") {
		t.Fatalf("launcher failed with a relative artifact path: %v\n%s", err, output)
	}
	files, err := os.ReadDir(buildTemp)
	if err != nil || len(files) != 0 {
		t.Fatalf("launcher left build artifacts: %v, %v", files, err)
	}
}
