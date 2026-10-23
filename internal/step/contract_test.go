package step

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This intentionally freezes the public names across the toolkit migration.
// It also checks that every YAML input is consumed and every export is declared.
func TestStepContract(t *testing.T) {
	data, err := os.ReadFile("../../step.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, contract, found := strings.Cut(string(data), "\ninputs:\n")
	if !found {
		t.Fatal("missing inputs")
	}
	inputs, outputsYAML, found := strings.Cut(contract, "\noutputs:\n")
	if !found {
		t.Fatal("missing outputs")
	}
	names := func(yaml string) []string {
		matches := regexp.MustCompile(`(?m)^  - (\w+):`).FindAllStringSubmatch(yaml, -1)
		var result []string
		for _, match := range matches {
			result = append(result, match[1])
		}
		sort.Strings(result)
		return result
	}
	wantInputs := []string{"app", "app_path", "firebase_token", "flags", "groups", "is_debug", "release_notes", "release_notes_file", "release_notes_length", "service_credentials_file", "testers", "upgrade_firebase_tools"}
	var consumed []string
	if _, err := ReadConfig(func(key string) string { consumed = append(consumed, key); return "" }); err != nil {
		t.Fatal(err)
	}
	sort.Strings(consumed)
	if !reflect.DeepEqual(names(inputs), wantInputs) || !reflect.DeepEqual(consumed, wantInputs) {
		t.Fatalf("input contract changed: declared=%v, consumed=%v", names(inputs), consumed)
	}
	wantOutputs := []string{"FIREBASE_APP_DISTRIBUTION_RELEASE_ID", "FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID", "FIREBASE_BITRISE_APP_DISTRIBUTION_URL", "FIREBASE_BITRISE_BINARY_DOWNLOAD_URL", "FIREBASE_BITRISE_CONSOLE_URL"}
	var exported []string
	for _, variable := range (outputs{}).variables() {
		exported = append(exported, variable.key)
	}
	sort.Strings(exported)
	if !reflect.DeepEqual(names(outputsYAML), wantOutputs) || !reflect.DeepEqual(exported, wantOutputs) {
		t.Fatalf("output contract changed: declared=%v, exported=%v", names(outputsYAML), exported)
	}
	for _, inputDefault := range []string{`app_path: "$BITRISE_IPA_PATH|$BITRISE_APK_PATH|$BITRISE_AAB_PATH"`, `upgrade_firebase_tools: "false"`, `is_debug: "false"`} {
		if !strings.Contains(inputs, inputDefault) {
			t.Errorf("input default changed: %s", inputDefault)
		}
	}
}
