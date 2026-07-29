// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/sanity"
)

const (
	// kindNetworkUpgradePrepare is the CR kind the execute_immediate.mf handler
	// creates.
	kindNetworkUpgradePrepare = "NetworkUpgradePrepare"

	// annotationCreatedBy records the component that authored a CR. The UC stamps
	// "UC" on every CR it creates so operators can distinguish UC-authored CRs
	// from daemon- or human-authored ones (HIP-1496).
	annotationCreatedBy = "operator.solo.hiero.org/created-by"
	createdByUC         = "UC"
)

// mintOperationID derives the upgrade operation id from the owning node, the
// consensus-node package version, and the execute_immediate.mf marker creation
// time: "<node>-upgrade-<version>-<YYYYMMDDTHHmmssZ>" (e.g.
// node0-upgrade-0.75.0-20260413T120000Z). All three inputs are stable across
// re-detections of the same marker on the same node, so recomputing yields the
// same id every tick — the UC "mints once" because the value is deterministic,
// not because it tracks state. The id is validated for path/shell safety (it
// flows into a persisted filename and a CR name).
func mintOperationID(version string, markerCreatedAt time.Time, consensusNode string) (string, error) {
	// Node is a PREFIX so the trailing "-<timestamp>" stays the last segment and
	// operationIDTimestamp keeps parsing it unchanged.
	id := fmt.Sprintf("%s-upgrade-%s-%s", consensusNode, version, markerCreatedAt.UTC().Format(sanity.TimestampLayout))
	if err := sanity.ValidateOperationID(id); err != nil {
		return "", errx.WithHints(
			errx.WithReason(errManifestInvalid.Wrap(err, "derived operationId %q is invalid", id), ReasonManifestInvalid),
			"Confirm images.consensusNode.version is a clean version string (no spaces or shell metacharacters)",
		)
	}
	// The operationId flows into two Kubernetes fields with different rules, so
	// validate both here — the single mint point shared by every name builder —
	// rather than failing opaquely at Create time:
	//
	//  1. It is used verbatim as the hedera.com/operation-id label value, which
	//     Kubernetes caps at 63 chars. That cap is why spec.operationId carries
	//     +kubebuilder:validation:MaxLength=63 (the CRD limit must not exceed the
	//     label-value limit); IsValidLabelValue enforces the same bound here.
	//  2. Its lowercased form is embedded in every CR metadata.name as
	//     "<operationId>-<step>". The longest step suffix ("-prepare"/"-execute")
	//     yields the longest name; if that is a valid DNS-1123 subdomain, every name
	//     builder's output is too. This also catches characters legal in an
	//     operationId but not a k8s name (notably '_'), plus the per-label and
	//     253-char length rules once the suffix is appended.
	if errs := k8svalidation.IsValidLabelValue(id); len(errs) > 0 {
		return "", errx.WithHints(
			errx.WithReason(
				errManifestInvalid.New("derived operationId %q is not a valid label value: %s", id, strings.Join(errs, "; ")),
				ReasonManifestInvalid),
			"Shorten images.consensusNode.version; the operationId (upgrade-<version>-<timestamp>) must be ≤63 characters",
		)
	}
	longestCRName := strings.ToLower(id) + "-" + operatorv1alpha1.StepPrepare
	if errs := k8svalidation.IsDNS1123Subdomain(longestCRName); len(errs) > 0 {
		return "", errx.WithHints(
			errx.WithReason(
				errManifestInvalid.New("derived CR name %q is not a valid Kubernetes name: %s", longestCRName, strings.Join(errs, "; ")),
				ReasonManifestInvalid),
			"Use a consensusNode version of only [A-Za-z0-9.-] (no underscores) that keeps the CR name within Kubernetes limits",
		)
	}
	return id, nil
}

// prepareCRName builds the RFC 1123-safe metadata.name for the Prepare CR:
// "<operationId-lowercased>-prepare". metadata.name carries no semantic meaning
// (operationId in the spec/label is the authoritative identity); it embeds the
// operationId so the name is a deterministic function of the upgrade: distinct
// upgrades never collide, and the same upgrade always maps to the same name — so a
// re-detected marker is a no-op (Create returns AlreadyExists) and a CR deleted
// then recreated (e.g. during DR) comes back under the same identity rather than as
// a second, parallel CR. CRs are retained after completion, but the naming does not
// depend on them being immortal.
func prepareCRName(operationID string) string {
	return strings.ToLower(operationID) + "-" + operatorv1alpha1.StepPrepare
}

// buildPrepareCR constructs the NetworkUpgradePrepare CR as an unstructured
// object ready for persist-before-send and the dynamic-client Create. spec.images
// is copied verbatim from the manifest; the UC does NOT do registry selection or
// layer-hash verification — that is the operator's PrepareReconciler.
func buildPrepareCR(cfg Config, operationID string, images operatorv1alpha1.UpgradeImagesSpec) (*unstructured.Unstructured, error) {
	prepare := &operatorv1alpha1.NetworkUpgradePrepare{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1alpha1.GroupVersion.String(),
			Kind:       kindNetworkUpgradePrepare,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prepareCRName(operationID),
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID: operationID,
				operatorv1alpha1.LabelStep:        operatorv1alpha1.StepPrepare,
			},
			Annotations: map[string]string{
				annotationCreatedBy: createdByUC,
			},
		},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			Orbit:       cfg.Orbit,
			OperationID: operationID,
			Images:      images,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(prepare)
	if err != nil {
		return nil, errx.WithReason(
			errCRBuild.Wrap(err, "convert NetworkUpgradePrepare %s to unstructured", operationID), ReasonCRBuildFailed)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}
