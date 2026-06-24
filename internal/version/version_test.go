// SPDX-License-Identifier: Apache-2.0

package version

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatJSON(t *testing.T) {
	info := Info{Version: "v1.2.3", Commit: "abc123", Date: "2026-01-01T00:00:00Z", GoVersion: "go1.26"}
	for _, format := range []string{"", "json", "JSON"} {
		out, err := info.Format(format)
		if err != nil {
			t.Fatalf("Format(%q): %v", format, err)
		}
		var got Info
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("Format(%q) produced invalid JSON %q: %v", format, out, err)
		}
		if got != info {
			t.Fatalf("Format(%q) round-trip = %+v, want %+v", format, got, info)
		}
	}
}

func TestFormatYAMLAndText(t *testing.T) {
	info := Info{Version: "v1.2.3", Commit: "abc123", Date: "2026-01-01T00:00:00Z", GoVersion: "go1.26"}

	yamlOut, err := info.Format("yaml")
	if err != nil {
		t.Fatalf("Format(yaml): %v", err)
	}
	if !strings.Contains(yamlOut, "version: v1.2.3") {
		t.Fatalf("yaml output missing version line: %q", yamlOut)
	}

	textOut, err := info.Format("text")
	if err != nil {
		t.Fatalf("Format(text): %v", err)
	}
	if !strings.Contains(textOut, "v1.2.3") || !strings.Contains(textOut, "abc123") {
		t.Fatalf("text output missing fields: %q", textOut)
	}
}

func TestFormatUnsupported(t *testing.T) {
	if _, err := (Info{}).Format("xml"); err == nil {
		t.Fatal("Format(xml) = nil error, want unsupported-format error")
	}
}

func TestGetDefaults(t *testing.T) {
	// Without -ldflags injection the build placeholders apply.
	got := Get()
	if got.Version != Number || got.Commit != Commit || got.Date != Date {
		t.Fatalf("Get() = %+v, want fields from package vars", got)
	}
	if got.GoVersion == "" {
		t.Fatal("Get().GoVersion is empty")
	}
}
