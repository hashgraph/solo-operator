// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const testRosterJSON = `{"nodeMetadata":[{"rosterEntry":{"nodeId":"0","weight":"1","gossipCaCertificate":"AQID","gossipEndpoint":[{"ipAddressV4":"CgAAAQ==","port":50111}]}}]}`

// writeRoster writes a roster file with the given content and mtime.
func writeRoster(t *testing.T, content string, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate-roster.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes roster: %v", err)
	}
	return path
}

func rosterCfg(path string, required bool) Config {
	return Config{CandidateRosterPath: path, CandidateRosterWait: 0, CandidateRosterRequired: required}
}

func TestReadCandidateRosterDisabled(t *testing.T) {
	art, err := readCandidateRoster(context.Background(), Config{}, markerCreatedAt)
	if err != nil || art != nil {
		t.Fatalf("disabled: got (%v, %v), want (nil, nil)", art, err)
	}
}

func TestReadCandidateRosterAttachesVerbatim(t *testing.T) {
	path := writeRoster(t, testRosterJSON, markerCreatedAt)

	art, err := readCandidateRoster(context.Background(), rosterCfg(path, true), markerCreatedAt)
	if err != nil {
		t.Fatalf("readCandidateRoster: %v", err)
	}
	if art.Name != operatorv1alpha1.ArtifactCandidateRoster {
		t.Fatalf("name = %q", art.Name)
	}
	if art.Content != testRosterJSON {
		t.Fatalf("content was altered: %q", art.Content)
	}
	sum := sha256.Sum256([]byte(testRosterJSON))
	if want := "sha256:" + hex.EncodeToString(sum[:]); string(art.Sha256) != want {
		t.Fatalf("sha256 = %q, want %q", art.Sha256, want)
	}
}

func TestReadCandidateRosterMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate-roster.json")

	// Not required: no artifact, no error.
	art, err := readCandidateRoster(context.Background(), rosterCfg(path, false), markerCreatedAt)
	if err != nil || art != nil {
		t.Fatalf("not required: got (%v, %v), want (nil, nil)", art, err)
	}

	// Required: a retryable error naming the reason.
	_, err = readCandidateRoster(context.Background(), rosterCfg(path, true), markerCreatedAt)
	if err == nil {
		t.Fatal("required: expected an error for a missing roster")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonCandidateRosterMissing {
		t.Fatalf("reason = %q, want %q", reason, ReasonCandidateRosterMissing)
	}
}

func TestReadCandidateRosterStale(t *testing.T) {
	// Written two hours before the marker: a leftover from an earlier upgrade.
	path := writeRoster(t, testRosterJSON, markerCreatedAt.Add(-2*time.Hour))

	art, err := readCandidateRoster(context.Background(), rosterCfg(path, false), markerCreatedAt)
	if err != nil || art != nil {
		t.Fatalf("not required: got (%v, %v), want (nil, nil)", art, err)
	}
	_, err = readCandidateRoster(context.Background(), rosterCfg(path, true), markerCreatedAt)
	if reason, _ := errx.ReasonOf(err); reason != ReasonCandidateRosterStale {
		t.Fatalf("reason = %q, want %q", reason, ReasonCandidateRosterStale)
	}
}

func TestReadCandidateRosterAcceptsFileWrittenJustBeforeOrAfterMarker(t *testing.T) {
	for _, offset := range []time.Duration{-5 * time.Minute, 0, 5 * time.Second} {
		path := writeRoster(t, testRosterJSON, markerCreatedAt.Add(offset))
		art, err := readCandidateRoster(context.Background(), rosterCfg(path, true), markerCreatedAt)
		if err != nil || art == nil {
			t.Fatalf("offset %s: got (%v, %v), want an artifact", offset, art, err)
		}
	}
}

func TestReadCandidateRosterInvalid(t *testing.T) {
	cases := map[string]string{
		"empty":        "",
		"not json":     "nodeMetadata: nope",
		"truncated":    `{"nodeMetadata":[`,
		"binary":       "\xff\xfe\x00{}",
		"over the cap": `"` + strings.Repeat("a", operatorv1alpha1.MaxArtifactContentBytes) + `"`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeRoster(t, content, markerCreatedAt)

			// Not required: skipped with a warning.
			art, err := readCandidateRoster(context.Background(), rosterCfg(path, false), markerCreatedAt)
			if err != nil || art != nil {
				t.Fatalf("not required: got (%v, %v), want (nil, nil)", art, err)
			}
			// Required: an error.
			if _, err := readCandidateRoster(context.Background(), rosterCfg(path, true), markerCreatedAt); err == nil {
				t.Fatal("required: expected an error for an invalid roster")
			}
		})
	}
}

func TestReadCandidateRosterSymlinkRejected(t *testing.T) {
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte(testRosterJSON), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(t.TempDir(), "candidate-roster.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err := readCandidateRoster(context.Background(), rosterCfg(link, true), markerCreatedAt)
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason = %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

func TestReadCandidateRosterWaitsForLateFile(t *testing.T) {
	old := candidateRosterPollInterval
	candidateRosterPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { candidateRosterPollInterval = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "candidate-roster.json")
	go func() {
		time.Sleep(60 * time.Millisecond)
		_ = os.WriteFile(path, []byte(testRosterJSON), 0o644)
	}()

	cfg := Config{CandidateRosterPath: path, CandidateRosterWait: 5 * time.Second, CandidateRosterRequired: true}
	art, err := readCandidateRoster(context.Background(), cfg, time.Now())
	if err != nil || art == nil {
		t.Fatalf("got (%v, %v), want the late file to be picked up", art, err)
	}
}

func TestReadCandidateRosterHonoursContextCancel(t *testing.T) {
	cfg := Config{
		CandidateRosterPath: filepath.Join(t.TempDir(), "candidate-roster.json"),
		CandidateRosterWait: time.Minute,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readCandidateRoster(ctx, cfg, markerCreatedAt); err == nil {
		t.Fatal("expected the cancelled context to stop the wait")
	}
}

func TestHandleExecuteImmediateAttachesCandidateRoster(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	cfg := testConfig(root, t.TempDir())
	cfg.CandidateRosterPath = writeRoster(t, testRosterJSON, markerCreatedAt)
	f := newFakeFactory(t, cfg)

	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("HandleExecuteImmediate: %v", err)
	}

	cr := getPrepare(t, f, wantOperationID)
	artifacts, found, _ := unstructured.NestedSlice(cr.Object, "spec", "artifacts")
	if !found || len(artifacts) != 1 {
		t.Fatalf("spec.artifacts = %v, want one entry", artifacts)
	}
	a := artifacts[0].(map[string]any)
	if a["name"] != operatorv1alpha1.ArtifactCandidateRoster || a["content"] != testRosterJSON {
		t.Fatalf("artifact not forwarded verbatim: %v", a)
	}
}

func TestHandleExecuteImmediateWithoutRosterStillCreatesPrepareCR(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	cfg := testConfig(root, t.TempDir())
	cfg.CandidateRosterPath = filepath.Join(t.TempDir(), "candidate-roster.json") // absent
	f := newFakeFactory(t, cfg)

	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("HandleExecuteImmediate: %v", err)
	}
	cr := getPrepare(t, f, wantOperationID)
	if _, found, _ := unstructured.NestedSlice(cr.Object, "spec", "artifacts"); found {
		t.Fatal("spec.artifacts must be absent when no roster was exported")
	}
}

func TestHandleExecuteImmediateRequiredRosterHoldsBackPrepareCR(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	cfg := testConfig(root, t.TempDir())
	cfg.CandidateRosterPath = filepath.Join(t.TempDir(), "candidate-roster.json") // absent
	cfg.CandidateRosterRequired = true
	f := newFakeFactory(t, cfg)

	if err := f.HandleExecuteImmediate(context.Background()); err == nil {
		t.Fatal("expected a retryable error while the required roster is missing")
	}
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("no CR should have been persisted without the required roster: %v", rest)
	}
}
