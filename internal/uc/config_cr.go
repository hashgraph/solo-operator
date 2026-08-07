// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// configSubdir is the subdirectory within the upgrade package that holds the
// ConsensusConfig files (HIP-1496 §§ Package Directory Structure).
const configSubdir = "data/config"

// kindEntry binds a package filename to its Kubernetes CR kind, GVR, and a
// short name suffix used to derive the CR metadata.name.
type kindEntry struct {
	gvr        schema.GroupVersionResource
	kind       string
	nameSuffix string
}

// configFileKinds is the hardcoded filename→CR-kind mapping for all recognised
// ConsensusConfig files. Files in data/config/ are keyed by their basename;
// package-root files (log4j2.xml, settings.txt) share the same namespace.
//
// Adding a new recognised filename requires only a new entry here plus its CR
// type in api/v1alpha1/ — no schema change needed (HIP-1496 §236).
var configFileKinds = map[string]kindEntry{
	// Package-root files.
	"log4j2.xml":   {gvr: log4j2ConfigGVR, kind: "Log4j2Config", nameSuffix: "log4j2config"},
	"settings.txt": {gvr: nodeSettingsGVR, kind: "NodeSettings", nameSuffix: "nodesettings"},

	// data/config/ files.
	"application.properties":          {gvr: applicationPropertiesGVR, kind: "ApplicationProperties", nameSuffix: "applicationproperties"},
	"application-override.properties": {gvr: applicationOverridePropertiesGVR, kind: "ApplicationOverrideProperties", nameSuffix: "applicationoverrideproperties"},
	"bootstrap.properties":            {gvr: bootstrapPropertiesGVR, kind: "BootstrapProperties", nameSuffix: "bootstrapproperties"},
	"node.properties":                 {gvr: nodePropertiesGVR, kind: "NodePropertiesConfig", nameSuffix: "nodepropertiesconfig"},
	"throttles.json":                  {gvr: throttlesConfigGVR, kind: "ThrottlesConfig", nameSuffix: "throttlesconfig"},
	"feeSchedules.json":               {gvr: feeSchedulesGVR, kind: "FeeSchedules", nameSuffix: "feeschedules"},
	"simpleFeesSchedules.json":        {gvr: simpleFeesSchedulesGVR, kind: "SimpleFeesSchedules", nameSuffix: "simplefeesschedules"},
	"api-permission.properties":       {gvr: apiPermissionPropertiesGVR, kind: "ApiPermissionProperties", nameSuffix: "apipermissionproperties"},
}

// packageRootFiles lists the filenames that live at the package root (not under
// data/config/). They are checked separately from the config subdirectory scan.
var packageRootFiles = []string{"log4j2.xml", "settings.txt"}

// ignoredConfigFilenames are files that legitimately ship in the upgrade
// package's data/config/ but are intentionally NOT turned into ConsensusConfig
// CRs, because the operator manages them by another mechanism. They are skipped
// (logged, not applied) rather than treated as unrecognised — this preserves the
// hard abort for genuinely unknown files while not choking on expected ones.
//
// Note: per-node block-node config (block-nodes/config/block-nodes-<NODE_ID>.json)
// and job descriptors (jobs/) live outside data/config/, so they are not scanned
// here and need no entry — only files that actually land in data/config/ do.
//
//   - genesis-network.json: a HIP-1494 data/config/ file (present for cluster-only
//     genesis), but it is the genesis roster / address book, which the operator
//     produces via the NetworkGenesis flow rather than a ConsensusConfig CR. The
//     package copy is skipped so it is not turned into a config CR.
var ignoredConfigFilenames = map[string]struct{}{
	"genesis-network.json": {},
}

// knownConfigFilenames returns a sorted slice of all filenames recognised by
// configFileKinds. Used in warning logs so operators can see the full list
// without consulting source code.
func knownConfigFilenames() []string {
	names := make([]string, 0, len(configFileKinds))
	for name := range configFileKinds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// scannedFile is a single recognised config file found in the upgrade package.
// Content is NOT stored here; it is read on demand in deployConfigCRs so that
// large or numerous files are never all held in memory simultaneously.
type scannedFile struct {
	absPath string
	entry   kindEntry
}

// scanConfigFiles walks the upgrade package and collects all recognised config
// files. It looks in two locations (per HIP-1496):
//   - <upgradePath>/data/config/ for most config files
//   - <upgradePath>/ (package root) for log4j2.xml and settings.txt
//
// Files whose names are not in configFileKinds are a terminal error — an
// unrecognised file would be silently unapplied, which is unsafe before a
// mainnet deploy. An absent data/config/ directory is not an error (some
// upgrades include no config changes).
func scanConfigFiles(upgradePath string) ([]scannedFile, error) {
	var files []scannedFile

	// Scan data/config/ (skipped cleanly if directory is absent).
	configDir := filepath.Join(upgradePath, configSubdir)
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			logx.As().Info().Str("reason", ReasonConfigDirAbsent.String()).
				Str("dir", configDir).Msg("data/config/ absent — no config CRs to deploy")
		} else {
			return nil, errx.WithHints(
				errx.WithReason(errPersist.Wrap(err, "read config dir %s", configDir), ReasonUpgradePathUnreadable),
				"Confirm the upgrade package was extracted correctly and data/config/ is readable",
			)
		}
	}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if _, skip := ignoredConfigFilenames[name]; skip {
			logx.As().Info().Str("reason", ReasonConfigFileIgnored.String()).
				Str("filename", name).Str("dir", configDir).
				Msg("config file is skipped (no config CR)")
			continue
		}
		entry, ok := configFileKinds[name]
		if !ok {
			logx.As().Error().Str("reason", ReasonConfigFileUnrecognised.String()).
				Str("filename", name).Str("dir", configDir).
				Strs("recognised", knownConfigFilenames()).
				Msg("unrecognised config filename — aborting execute phase")
			return nil, errx.WithHints(
				errx.WithReason(errPersist.New("unrecognised config filename %q in %s", name, configDir), ReasonConfigFileUnrecognised),
				"Remove or rename the unrecognised file from data/config/ in the upgrade package",
				"Recognised filenames: "+strings.Join(knownConfigFilenames(), ", "),
			)
		}
		files = append(files, scannedFile{absPath: filepath.Join(configDir, name), entry: entry})
	}

	// Scan package-root files (log4j2.xml, settings.txt).
	for _, name := range packageRootFiles {
		path := filepath.Join(upgradePath, name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue // optional — not all packages include these files
			}
			return nil, errx.WithHints(
				errx.WithReason(errPersist.Wrap(err, "stat %s", path), ReasonUpgradePathUnreadable),
				"Confirm the upgrade package was extracted correctly",
			)
		}
		files = append(files, scannedFile{absPath: path, entry: configFileKinds[name]})
	}

	return files, nil
}

// configCRName derives the metadata.name for a config CR. The name encodes the
// operationId (for uniqueness across sequential upgrades — CRs are never deleted)
// and the kind suffix (for uniqueness within one upgrade operation).
// Format: "<operationId-lowercased>-<nameSuffix>".
func configCRName(operationID string, entry kindEntry) string {
	return strings.ToLower(operationID) + "-" + entry.nameSuffix
}

// buildConfigCR reads absPath and constructs an *unstructured.Unstructured
// ready to be created in the cluster. spec.orbit links it to the owning Orbit
// for cascade deletion. The file is read here, one CR at a time, so content is
// never buffered across multiple files simultaneously.
func buildConfigCR(cfg ProvisionerProxyConfig, operationID string, sf scannedFile) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(sf.absPath)
	if err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "read config file %s", sf.absPath), ReasonUpgradePathUnreadable),
			"Confirm the upgrade package was extracted correctly and the file is readable",
		)
	}

	name := configCRName(operationID, sf.entry)
	cr := &unstructured.Unstructured{}
	cr.SetAPIVersion(operatorv1alpha1.GroupVersion.String())
	cr.SetKind(sf.entry.kind)
	cr.SetName(name)
	cr.SetNamespace(cfg.Namespace)
	cr.SetLabels(map[string]string{
		operatorv1alpha1.LabelOperationID: operationID,
	})
	cr.SetAnnotations(map[string]string{
		annotationCreatedBy: createdByUC,
	})
	_ = unstructured.SetNestedField(cr.Object, string(data), "spec", "content")
	if cfg.Orbit != "" {
		_ = unstructured.SetNestedField(cr.Object, cfg.Orbit, "spec", "orbit")
	}
	return cr, nil
}

// configCRRef binds a created config CR's name to its GVR so it can be watched.
type configCRRef struct {
	name string
	gvr  schema.GroupVersionResource
	kind string
}

// deployConfigCRs scans the upgrade package for recognised config files, creates
// the corresponding CRs (idempotent — AlreadyExists is success), and waits for
// the operator to reconcile each one to Valid=True before returning.
//
// The caller provides a context already scoped to configCRDeployTimeout. If no
// config files are found the function returns nil immediately.
func (p *ProvisionerProxy) deployConfigCRs(ctx context.Context, operationID string) error {
	if p.cfg.UpgradePath == "" {
		return errx.WithHints(
			errx.WithReason(errPersist.New("UpgradePath is empty — cannot deploy config CRs"), ReasonUpgradePathUnreadable),
			"Ensure --upgrade-path is set in the UC configuration",
		)
	}

	scanned, err := scanConfigFiles(p.cfg.UpgradePath)
	if err != nil {
		return err
	}

	refs := make([]configCRRef, 0, len(scanned)+1)
	for _, sf := range scanned {
		cr, err := buildConfigCR(p.cfg, operationID, sf)
		if err != nil {
			return err
		}

		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		_, createErr := p.client.Resource(sf.entry.gvr).Namespace(p.cfg.Namespace).
			Create(createCtx, cr, metav1.CreateOptions{})
		cancel()

		if createErr != nil {
			if k8serrors.IsAlreadyExists(createErr) {
				logx.As().Debug().Str("reason", ReasonCRExists.String()).
					Str("operation_id", operationID).Str("cr_kind", sf.entry.kind).
					Str("cr_name", cr.GetName()).Msg("config CR already exists — skipping create")
			} else {
				return errx.WithHints(
					errx.WithReason(errK8sAPI.Wrap(createErr, "create %s %s", sf.entry.kind, cr.GetName()), ReasonConfigCRCreateFailed),
					"Confirm the UC ServiceAccount has create permission on "+sf.entry.gvr.Resource+"."+sf.entry.gvr.Group,
					"Check API server connectivity and that the CRD is installed",
				)
			}
		} else {
			logx.As().Info().Str("reason", ReasonCRDelivered.String()).
				Str("operation_id", operationID).Str("cr_kind", sf.entry.kind).
				Str("cr_name", cr.GetName()).Msg("created config CR")
		}

		refs = append(refs, configCRRef{name: cr.GetName(), gvr: sf.entry.gvr, kind: sf.entry.kind})
	}

	// Per-node block-node config lives outside data/config/ (block-nodes/config/)
	// and is selected by NODE_ID (HIP-1494), so it is deployed separately.
	bnRef, err := p.deployBlockNodesConfigCR(ctx, operationID)
	if err != nil {
		return err
	}
	if bnRef != nil {
		refs = append(refs, *bnRef)
	}

	if len(refs) == 0 {
		logx.As().Info().Str("reason", ReasonConfigDirAbsent.String()).
			Str("operation_id", operationID).
			Msg("no config files in upgrade package — skipping config CR deployment")
		return nil
	}

	return waitForConfigCRsValid(ctx, p.client, p.cfg.Namespace, operationID, refs, p.cfg.pollInterval())
}

// blockNodesSubdir is the package subdirectory holding per-node block-node config
// files (block-nodes-<NODE_ID>.json), separate from data/config/ (HIP-1494).
const blockNodesSubdir = "block-nodes/config"

// deployBlockNodesConfigCR selects this node's block-node config from the upgrade
// package (block-nodes/config/block-nodes-<NODE_ID>.json) and upserts it verbatim
// into a BlockNodesConfig CR named "<capsule>-block-nodes". That name is stable
// across upgrades so the ConsensusCapsule can reference it via a static
// spec.blockNodesConfigRef (config refs are not patched during upgrade); a
// re-upgrade therefore overwrites the content rather than being a no-op. Returns
// (nil, nil) when the package carries no block-node config for this node, or when
// the capsule name is unknown (nothing to reference).
func (p *ProvisionerProxy) deployBlockNodesConfigCR(ctx context.Context, operationID string) (*configCRRef, error) {
	if p.cfg.CapsuleName == "" {
		return nil, nil
	}
	path := filepath.Join(p.cfg.UpgradePath, blockNodesSubdir, fmt.Sprintf("block-nodes-%d.json", p.cfg.NodeID))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logx.As().Info().Str("reason", ReasonConfigDirAbsent.String()).
				Str("operation_id", operationID).Str("path", path).
				Msg("no per-node block-node config in upgrade package — skipping")
			return nil, nil
		}
		return nil, errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "read block-node config %s", path), ReasonUpgradePathUnreadable),
			"Confirm the upgrade package was extracted correctly and block-nodes/config/ is readable",
		)
	}

	name := p.cfg.CapsuleName + "-block-nodes"
	cr := &unstructured.Unstructured{}
	cr.SetAPIVersion(operatorv1alpha1.GroupVersion.String())
	cr.SetKind("BlockNodesConfig")
	cr.SetName(name)
	cr.SetNamespace(p.cfg.Namespace)
	cr.SetLabels(map[string]string{
		operatorv1alpha1.LabelOperationID: operationID,
	})
	cr.SetAnnotations(map[string]string{
		annotationCreatedBy: createdByUC,
	})
	_ = unstructured.SetNestedField(cr.Object, string(data), "spec", "content")
	if p.cfg.Orbit != "" {
		_ = unstructured.SetNestedField(cr.Object, p.cfg.Orbit, "spec", "orbit")
	}

	if err := p.upsertConfigCR(ctx, blockNodesConfigGVR, cr); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "apply BlockNodesConfig %s", name), ReasonConfigCRCreateFailed),
			"Confirm the UC ServiceAccount has create/update permission on blocknodesconfigs.operator.solo.hiero.org",
		)
	}
	logx.As().Info().Str("reason", ReasonCRDelivered.String()).
		Str("operation_id", operationID).Str("cr_kind", "BlockNodesConfig").
		Str("cr_name", name).Int("node_id", p.cfg.NodeID).
		Msg("staged per-node block-node config CR")
	return &configCRRef{name: name, gvr: blockNodesConfigGVR, kind: "BlockNodesConfig"}, nil
}

// upsertConfigCR creates cr, or updates the existing object's spec when it already
// exists. Used for the stable-named block-node CR whose content must track the
// latest upgrade package.
func (p *ProvisionerProxy) upsertConfigCR(ctx context.Context, gvr schema.GroupVersionResource, cr *unstructured.Unstructured) error {
	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	_, err := p.client.Resource(gvr).Namespace(p.cfg.Namespace).Create(createCtx, cr, metav1.CreateOptions{})
	cancel()
	if err == nil {
		return nil
	}
	if !k8serrors.IsAlreadyExists(err) {
		return err
	}

	getCtx, cancel := context.WithTimeout(ctx, listTimeout)
	existing, getErr := p.client.Resource(gvr).Namespace(p.cfg.Namespace).Get(getCtx, cr.GetName(), metav1.GetOptions{})
	cancel()
	if getErr != nil {
		return getErr
	}
	spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if err := unstructured.SetNestedMap(existing.Object, spec, "spec"); err != nil {
		return err
	}
	existing.SetLabels(cr.GetLabels())
	updateCtx, cancel := context.WithTimeout(ctx, createTimeout)
	_, updErr := p.client.Resource(gvr).Namespace(p.cfg.Namespace).Update(updateCtx, existing, metav1.UpdateOptions{})
	cancel()
	return updErr
}

// waitForConfigCRsValid polls each config CR's status.conditions until all have
// status.conditions[type=Valid].status == "True". It returns nil only when every
// CR has passed operator validation.
//
// A CR whose Valid condition is False is a terminal failure: the operator has
// rejected the file content. The function logs ERROR and returns an error so the
// caller does NOT advance to PendingNodeUpgrade.
//
// Context cancellation (including configCRDeployTimeout) causes a timeout error,
// leaving the Execute CR at ReadyForProvisionerDaemon for operator intervention.
func waitForConfigCRsValid(ctx context.Context, client dynamic.Interface, namespace, operationID string, refs []configCRRef, pollInterval time.Duration) error {
	pending := make(map[string]configCRRef, len(refs))
	for _, r := range refs {
		pending[r.name] = r
	}

	// checkPending gets each pending CR once and advances the map. Returns a
	// terminal error if any CR is Valid=False; nil otherwise (callers re-enter
	// on the next tick if pending is still non-empty).
	checkPending := func() error {
		for name, ref := range pending {
			getCtx, cancel := context.WithTimeout(ctx, listTimeout)
			obj, err := client.Resource(ref.gvr).Namespace(namespace).Get(getCtx, name, metav1.GetOptions{})
			cancel()
			if err != nil {
				logx.As().Warn().Err(err).Str("reason", ReasonConfigCRGetFailed.String()).
					Str("operation_id", operationID).Str("cr_kind", ref.kind).Str("cr_name", name).
					Msg("could not get config CR — will retry")
				continue
			}

			condStatus, condFound := conditionStatus(obj, "Valid")
			if !condFound {
				logx.As().Debug().
					Str("operation_id", operationID).Str("cr_kind", ref.kind).Str("cr_name", name).
					Msg("config CR has no Valid condition yet — waiting")
				continue
			}

			switch condStatus {
			case string(metav1.ConditionTrue):
				logx.As().Info().Str("reason", ReasonConfigCRsDeployed.String()).
					Str("operation_id", operationID).Str("cr_kind", ref.kind).Str("cr_name", name).
					Msg("config CR reconciled — Valid=True")
				delete(pending, name)

			case string(metav1.ConditionFalse):
				msg := conditionMessage(obj, "Valid")
				logx.As().Error().Str("reason", ReasonConfigCRFailed.String()).
					Str("operation_id", operationID).Str("cr_kind", ref.kind).Str("cr_name", name).
					Str("condition_message", msg).
					Msg("config CR is invalid — operator rejected content, aborting execute phase")
				return errx.WithHints(
					errx.WithReason(errK8sAPI.New("%s %s is Valid=False: %s", ref.kind, name, msg), ReasonConfigCRFailed),
					"Inspect the "+ref.kind+" CR status and fix the config file content in the upgrade package",
					"Delete the config CR and the now_frozen.mf marker once the content is corrected",
				)
			}
		}
		return nil
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for len(pending) > 0 {
		// Immediate first check before waiting for the ticker — avoids a full
		// poll-interval delay when CRs are already Valid (common in tests
		// and fast-reconciling clusters).
		if err := checkPending(); err != nil {
			return err
		}
		if len(pending) == 0 {
			break
		}

		select {
		case <-ctx.Done():
			names := make([]string, 0, len(pending))
			for n := range pending {
				names = append(names, n)
			}
			return errx.WithHints(
				errx.WithReason(errK8sAPI.New("timed out waiting for config CRs to become valid: %v", names), ReasonConfigCRWaitTimeout),
				"Check operator logs — config CR reconciliation may have stalled",
				"Confirm the UC ServiceAccount has get permission on the config CR resources",
			)
		case <-ticker.C:
			if err := checkPending(); err != nil {
				return err
			}
		}
	}

	logx.As().Info().Str("reason", ReasonConfigCRsDeployed.String()).
		Str("operation_id", operationID).Int("count", len(refs)).
		Msg("all config CRs are Valid — proceeding to PendingNodeUpgrade")
	return nil
}

// conditionMessage returns the message field of the named condition, or "" if
// no matching condition exists.
func conditionMessage(obj *unstructured.Unstructured, condType string) string {
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		return ""
	}
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		if t != condType {
			continue
		}
		m, _, _ := unstructured.NestedString(cond, "message")
		return m
	}
	return ""
}

// conditionStatus returns the status string ("True"/"False"/"Unknown") and
// whether it found a condition of the given type in the CR's
// status.conditions list.
func conditionStatus(obj *unstructured.Unstructured, condType string) (string, bool) {
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		return "", false
	}
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		if t != condType {
			continue
		}
		s, _, _ := unstructured.NestedString(cond, "status")
		return s, true
	}
	return "", false
}
