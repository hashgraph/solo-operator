// SPDX-License-Identifier: Apache-2.0

// Package version exposes the running binary's build metadata (version, commit,
// build date) and a Cobra "version" command. Values are injected at build time
// via -ldflags -X (see the Taskfile build tasks); they fall back to development
// placeholders for `go run` / `go build` without ldflags.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// Build metadata, set via -ldflags "-X github.com/hashgraph/solo-operator/internal/version.<var>=...".
var (
	// Number is the semantic version (e.g. v1.2.3) or "dev" for unstamped builds.
	Number = "dev"
	// Commit is the git commit hash the binary was built from.
	Commit = "none"
	// Date is the build timestamp (RFC 3339).
	Date = "unknown"
)

// Output formats supported by the version command's --output flag.
const (
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatText = "text"
)

// Info is the build metadata for the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"goVersion"`
}

// Get returns the running binary's Info.
func Get() Info {
	return Info{
		Version:   Number,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
	}
}

// Format renders the Info in the requested format. An empty format defaults to JSON.
func (i Info) Format(format string) (string, error) {
	switch strings.ToLower(format) {
	case "", FormatJSON:
		out, err := json.Marshal(i)
		if err != nil {
			return "", fmt.Errorf("marshal version to json: %w", err)
		}
		return string(out), nil
	case FormatYAML:
		out, err := yaml.Marshal(i)
		if err != nil {
			return "", fmt.Errorf("marshal version to yaml: %w", err)
		}
		return strings.TrimRight(string(out), "\n"), nil
	case FormatText:
		return i.Text(), nil
	default:
		return "", fmt.Errorf("unsupported format %q (want json, yaml, or text)", format)
	}
}

// Text returns a human-readable, multi-line version string.
func (i Info) Text() string {
	return fmt.Sprintf("Version:    %s\nCommit:     %s\nBuilt:      %s\nGo Version: %s",
		i.Version, i.Commit, i.Date, i.GoVersion)
}

// Cmd returns a "version" Cobra command. It honours an inherited --output flag
// (json, yaml, or text) and falls back to text when the flag is absent.
func Cmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information and exit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format == "" {
				if inherited, err := cmd.Flags().GetString("output"); err == nil && inherited != "" {
					format = inherited
				} else {
					format = FormatText
				}
			}
			out, err := Get().Format(format)
			if err != nil {
				return err
			}
			cmd.Println(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: text, json, or yaml")
	return cmd
}
