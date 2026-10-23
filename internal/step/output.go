package step

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

type release struct {
	Name              string `json:"name"`
	ConsoleURL        string `json:"firebaseConsoleUri"`
	TestingURL        string `json:"testingUri"`
	BinaryDownloadURL string `json:"binaryDownloadUri"`
}

type outputs struct {
	ConsoleURL        string
	TestingURL        string
	BinaryDownloadURL string
	ReleaseID         string
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func parseOutput(raw string) outputs {
	lines := strings.Split(ansiEscape.ReplaceAllString(raw, ""), "\n")
	var result outputs
	var fallback outputs
	for _, line := range lines {
		if marker := strings.Index(line, "[apiv2][body]"); marker >= 0 {
			body := line[marker+len("[apiv2][body]"):]
			if start := strings.IndexByte(body, '{'); start >= 0 {
				var operation struct {
					Done     bool `json:"done"`
					Response struct {
						Release *release `json:"release"`
					} `json:"response"`
				}
				if json.Unmarshal([]byte(body[start:]), &operation) == nil && operation.Done && operation.Response.Release != nil {
					r := operation.Response.Release
					result = outputs{r.ConsoleURL, r.TestingURL, r.BinaryDownloadURL, releaseID(r.Name)}
				}
			}
		}
		for _, labeled := range []struct {
			label string
			value *string
		}{
			{"View this release in the Firebase console:", &fallback.ConsoleURL},
			{"Share this release with testers who have access:", &fallback.TestingURL},
			{"Download the release binary (link expires in 1 hour):", &fallback.BinaryDownloadURL},
		} {
			if i := strings.Index(line, labeled.label); i >= 0 {
				if fields := strings.Fields(line[i+len(labeled.label):]); len(fields) > 0 {
					*labeled.value = fields[0]
				}
			}
		}
	}
	if result.ConsoleURL == "" {
		result.ConsoleURL = fallback.ConsoleURL
	}
	if result.TestingURL == "" {
		result.TestingURL = fallback.TestingURL
	}
	if result.BinaryDownloadURL == "" {
		result.BinaryDownloadURL = fallback.BinaryDownloadURL
	}
	if result.ReleaseID == "" {
		if u, err := url.Parse(result.ConsoleURL); err == nil {
			result.ReleaseID = releaseID(u.Path)
		}
	}
	return result
}

func releaseID(name string) string {
	const marker = "/releases/"
	if i := strings.LastIndex(name, marker); i >= 0 {
		id := name[i+len(marker):]
		if id != "" && !strings.Contains(id, "/") {
			return id
		}
	}
	return ""
}

func (o outputs) variables() []struct{ key, value string } {
	return []struct{ key, value string }{
		{"FIREBASE_BITRISE_CONSOLE_URL", o.ConsoleURL},
		{"FIREBASE_BITRISE_APP_DISTRIBUTION_URL", o.TestingURL},
		{"FIREBASE_BITRISE_BINARY_DOWNLOAD_URL", o.BinaryDownloadURL},
		{"FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID", o.ReleaseID},
		{"FIREBASE_APP_DISTRIBUTION_RELEASE_ID", o.ReleaseID},
	}
}

func displayOutput(raw string, debug, failed bool) string {
	if debug || failed {
		return raw
	}
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		plain := ansiEscape.ReplaceAllString(line, "")
		for _, prefix := range []string{"✔", "i ", "⚠", "!", "Error:"} {
			if strings.HasPrefix(plain, prefix) {
				lines = append(lines, line)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}
