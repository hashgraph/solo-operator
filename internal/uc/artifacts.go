// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"time"
	"unicode/utf8"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// candidateRosterPollInterval is how often readCandidateRoster re-checks for the
// file while waiting. A var only so tests can shrink it; production never reassigns it.
var candidateRosterPollInterval = 500 * time.Millisecond

// candidateRosterMaxAge is how much older than execute_immediate.mf the roster
// file may be and still count as belonging to this upgrade. The consensus node
// exports the roster during PREPARE_UPGRADE handling, so the file is written within
// moments of the marker (before it, while the package extracts, or just after). A
// file much older than the marker is a leftover from an earlier upgrade, because the
// file lives outside current/ and is not wiped with it.
const candidateRosterMaxAge = 30 * time.Minute

// rosterOutcome classifies one attempt to read the candidate roster.
type rosterOutcome int

const (
	rosterOK rosterOutcome = iota
	// rosterMissing: no file yet. Retried until the wait elapses.
	rosterMissing
	// rosterStale: a file exists but predates this upgrade. Retried, because the
	// node may be about to overwrite it.
	rosterStale
	// rosterInvalid: the file is unusable (symlink, oversized, empty, not UTF-8 JSON).
	// Terminal; waiting will not fix it.
	rosterInvalid
)

// readCandidateRoster reads the candidate-roster.json the consensus node exported at
// PREPARE_UPGRADE and returns it as an opaque artifact for the Prepare CR. The UC
// does not interpret the roster: it only checks that the file is a regular, bounded,
// UTF-8 JSON document and records its sha256, so the operator can verify it.
//
// markerTime is execute_immediate.mf's mtime; it anchors the freshness check.
//
// A nil artifact with a nil error means "no roster to attach" (feature disabled, or
// the file never appeared and the roster is not required); the Prepare CR is then
// created without it. When cfg.CandidateRosterRequired is set, a missing, stale or
// invalid roster returns an error instead, so the handler retries on the next poll
// tick and no Prepare CR is created without it.
func readCandidateRoster(ctx context.Context, cfg Config, markerTime time.Time) (*operatorv1alpha1.UpgradeArtifact, error) {
	path := cfg.CandidateRosterPath
	if path == "" {
		return nil, nil
	}

	deadline := time.Now().Add(cfg.CandidateRosterWait)
	for {
		art, outcome, err := tryReadCandidateRoster(path, markerTime)
		switch outcome {
		case rosterOK:
			logx.As().Info().Str("reason", ReasonCandidateRosterAttached.String()).
				Str("path", path).Str("sha256", string(art.Sha256)).Int("bytes", len(art.Content)).
				Msg("Attaching candidate roster to the Prepare CR")
			return art, nil
		case rosterInvalid:
			return nil, rosterUnavailable(cfg, path, err)
		}
		if !time.Now().Before(deadline) {
			return nil, rosterUnavailable(cfg, path, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(candidateRosterPollInterval):
		}
	}
}

// rosterUnavailable decides what a roster that could not be read means: an error
// when the roster is required, otherwise a logged warning and no artifact.
func rosterUnavailable(cfg Config, path string, err error) error {
	if cfg.CandidateRosterRequired {
		return err
	}
	logx.As().Warn().Err(err).Str("path", path).
		Msg("Creating the Prepare CR without a candidate roster; roster-driven changes (key rotation, firewall) will not be planned for this upgrade")
	return nil
}

// tryReadCandidateRoster makes one attempt. The returned error is already decorated
// with a reason and hints; it is nil only for rosterOK.
func tryReadCandidateRoster(path string, markerTime time.Time) (*operatorv1alpha1.UpgradeArtifact, rosterOutcome, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, rosterMissing, errx.WithHints(
				errx.WithReason(errPersist.New("candidate roster %s does not exist", path), ReasonCandidateRosterMissing),
				"Confirm the consensus node sets networkAdmin.candidateRosterExportFile to data/upgrade/candidate-roster.json, and exportCandidateRoster / createCandidateRosterOnPrepareUpgrade are true",
			)
		}
		return nil, rosterMissing, errx.WithReason(errPersist.Wrap(err, "stat %s", path), ReasonUpgradePathUnreadable)
	}
	if err := rejectSymlink(path, info.Mode()); err != nil {
		return nil, rosterInvalid, err
	}
	if err := rejectOversized(path, info.Size()); err != nil {
		return nil, rosterInvalid, err
	}
	if info.ModTime().Before(markerTime.Add(-candidateRosterMaxAge)) {
		return nil, rosterStale, errx.WithHints(
			errx.WithReason(
				errPersist.New("candidate roster %s (modified %s) predates this upgrade (marker %s)",
					path, info.ModTime().UTC().Format(time.RFC3339), markerTime.UTC().Format(time.RFC3339)),
				ReasonCandidateRosterStale),
			"The file is a leftover from an earlier upgrade; the consensus node did not export a roster for this one",
		)
	}

	data, err := readFileNoFollow(path)
	if err != nil {
		if isDecorated(err) {
			return nil, rosterInvalid, err
		}
		return nil, rosterMissing, errx.WithReason(errPersist.Wrap(err, "read %s", path), ReasonUpgradePathUnreadable)
	}
	if len(data) == 0 || len(data) > operatorv1alpha1.MaxArtifactContentBytes || !utf8.Valid(data) || !json.Valid(data) {
		return nil, rosterInvalid, errx.WithHints(
			errx.WithReason(
				errConfigFileInvalid.New("candidate roster %s is empty, larger than %d bytes, or not valid UTF-8 JSON",
					path, operatorv1alpha1.MaxArtifactContentBytes),
				ReasonCandidateRosterInvalid),
			"Check the consensus node's candidate-roster export",
		)
	}

	sum := sha256.Sum256(data)
	return &operatorv1alpha1.UpgradeArtifact{
		Name:    operatorv1alpha1.ArtifactCandidateRoster,
		Sha256:  operatorv1alpha1.Sha256Digest("sha256:" + hex.EncodeToString(sum[:])),
		Content: string(data),
	}, rosterOK, nil
}
