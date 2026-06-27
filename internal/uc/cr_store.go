// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/automa-saga/logx"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/sanity"
)

// CRStore implements the persist-before-send primitive for crash safety
// (HIP-1496 §"Marker File Processing Lifecycle" steps 3–4 + §"Idempotent
// Recovery on Restart"). Each constructed NetworkUpgrade* CR is serialized to
// YAML under --cr-persist-path BEFORE the K8s API Create, and the file is
// removed only after the Create succeeds. A file that survives a restart marks
// the crash window between persist and send: it is re-read on startup and
// re-sent by the recovery flow (#998). This story owns the write/delete
// primitive and the on-disk format; it does not own the re-send flow itself.
//
// Files are named "<operationID>.<step>.yaml". The name is deterministic from
// the operationId and step so recovery can correlate a persisted file with the
// upgrade operation and the marker that produced it, without parsing the body.
type CRStore struct {
	dir string
}

// NewCRStore returns a store that persists CRs under dir (--cr-persist-path).
func NewCRStore(dir string) *CRStore {
	return &CRStore{dir: dir}
}

// PersistedCR is one CR found on disk during recovery, with the operationId and
// step recovered from its filename and the object decoded from its body.
type PersistedCR struct {
	OperationID string
	Step        string
	Path        string
	Object      *unstructured.Unstructured
}

// validSteps is the closed set of step values, matching the LabelStep values the
// handlers stamp on each CR (api/v1alpha1). A step outside this set is rejected
// so a malformed filename can never be silently treated as a recoverable CR.
var validSteps = map[string]struct{}{
	operatorv1alpha1.StepPrepare: {},
	operatorv1alpha1.StepFreeze:  {},
	operatorv1alpha1.StepExecute: {},
	operatorv1alpha1.StepAbort:   {},
}

// fileExt is the on-disk extension for persisted CRs.
const fileExt = ".yaml"

// Persist serializes cr to YAML under the store directory before the K8s API
// send and returns the file path. It is idempotent: an existing file for the
// same operationId+step is overwritten (re-detection of the same marker must
// re-persist the same CR, not fail). The directory is created if absent.
func (s *CRStore) Persist(operationID, step string, cr *unstructured.Unstructured) (string, error) {
	name, err := fileName(operationID, step)
	if err != nil {
		return "", err
	}
	if cr == nil || cr.Object == nil {
		return "", errx.WithReason(
			errCRBuild.New("cannot persist a nil CR for operationId %q step %q", operationID, step), ReasonCRBuildFailed)
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return "", errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "create cr-persist dir %s", s.dir), ReasonCRPersistWriteFailed),
			"Confirm --cr-persist-path is on a writable volume with free space",
		)
	}
	data, err := yaml.Marshal(cr.Object)
	if err != nil {
		return "", errx.WithReason(errCRBuild.Wrap(err, "marshal CR %s/%s", operationID, step), ReasonCRBuildFailed)
	}
	path := filepath.Join(s.dir, name)
	// Write atomically: write a temp file then rename, so a crash mid-write can
	// never leave a truncated CR on disk that recovery would try to re-send.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return "", errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "write CR %s", tmp), ReasonCRPersistWriteFailed),
			"Confirm --cr-persist-path is on a writable volume with free space",
		)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", errx.WithReason(errPersist.Wrap(err, "rename CR %s -> %s", tmp, path), ReasonCRPersistWriteFailed)
	}
	return path, nil
}

// Delete removes the persisted CR for operationId+step after a successful
// Create. A missing file is not an error: deletion is the success path and must
// be safe to call after a restart-recovery re-send where the file may already
// be gone.
func (s *CRStore) Delete(operationID, step string) error {
	name, err := fileName(operationID, step)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !os.IsNotExist(err) {
		return errx.WithReason(errPersist.Wrap(err, "delete persisted CR %s", name), ReasonCRPersistDeleteFailed)
	}
	return nil
}

// List re-reads the store directory and returns every persisted CR, decoded
// from disk and sorted by filename for deterministic recovery order. A missing
// directory yields no entries and no error (nothing was ever persisted). This
// is the read half of restart recovery; the actual re-send/cleanup decision
// belongs to #998.
func (s *CRStore) List() ([]PersistedCR, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errx.WithReason(errPersist.Wrap(err, "read cr-persist dir %s", s.dir), ReasonCRPersistListFailed)
	}

	var out []PersistedCR
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), fileExt) {
			continue
		}
		operationID, step, ok := parseFileName(e.Name())
		if !ok {
			// A file that does not match the deterministic naming scheme (e.g. a
			// leftover ".tmp" or a foreign file) is not a CR this store wrote; skip
			// it rather than fail the whole recovery scan.
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, errx.WithReason(errPersist.Wrap(err, "read persisted CR %s", path), ReasonCRPersistReadFailed)
		}
		obj := map[string]interface{}{}
		if err := yaml.Unmarshal(data, &obj); err != nil {
			// A single corrupt file must not halt the whole recovery scan — that would
			// retry-loop the handler forever. Skip it with a warning so the remaining
			// CRs still recover; the operator can remove the bad file out-of-band.
			common.LogErr(
				logx.As().Warn(),
				errx.WithHints(
					errx.WithReason(errPersist.Wrap(err, "unmarshal persisted CR %s", path), ReasonCRPersistCorrupt),
					"A persisted CR file is corrupt; remove it from --cr-persist-path to let recovery proceed",
				),
				ReasonCRPersistCorrupt,
			).Str("path", path).Msg("skipping corrupt persisted CR")
			continue
		}
		out = append(out, PersistedCR{
			OperationID: operationID,
			Step:        step,
			Path:        path,
			Object:      &unstructured.Unstructured{Object: obj},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// fileName builds the deterministic "<operationID>.<step>.yaml" filename. The
// operationId is validated for path/shell safety (it flows into a filename) and
// the step must be one of the known values, so a bad input is rejected at the
// boundary rather than producing a file recovery cannot correlate.
func fileName(operationID, step string) (string, error) {
	if err := sanity.ValidateOperationID(operationID); err != nil {
		// Reason-code even this "should never happen" input so callers on the
		// persist-before-send boundary always get an enumerable, log-correlatable
		// error rather than a bare validation message.
		return "", errx.WithReason(errCRBuild.Wrap(err, "invalid operationId %q", operationID), ReasonCRBuildFailed)
	}
	if _, ok := validSteps[step]; !ok {
		return "", errx.WithReason(errCRBuild.New("unknown CR step %q", step), ReasonCRBuildFailed)
	}
	return operationID + "." + step + fileExt, nil
}

// parseFileName recovers the operationId and step from a persisted filename. The
// step is the final dot-separated segment before the extension (step values
// contain no dots); the operationId is everything before it (it may itself
// contain dots, e.g. an embedded version "upgrade-v0.76.0-<ts>"). It returns ok
// only for a well-formed name with a known step and a path/shell-safe
// operationId — the same checks fileName enforces on write.
func parseFileName(name string) (operationID, step string, ok bool) {
	base := strings.TrimSuffix(name, fileExt)
	if base == name {
		return "", "", false
	}
	idx := strings.LastIndex(base, ".")
	if idx <= 0 || idx == len(base)-1 {
		return "", "", false
	}
	operationID, step = base[:idx], base[idx+1:]
	if _, ok := validSteps[step]; !ok {
		return "", "", false
	}
	if sanity.ValidateOperationID(operationID) != nil {
		return "", "", false
	}
	return operationID, step, true
}
