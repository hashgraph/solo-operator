// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

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

// mintOperationID derives the upgrade operation id from the consensus-node
// package version and the execute_immediate.mf marker creation time, per
// HIP-1496: "upgrade-<version>-<YYYYMMDDTHHmmssZ>" (e.g.
// upgrade-0.75.0-20260413T120000Z). Both inputs are stable across re-detections
// of the same marker, so recomputing yields the same id every tick — the UC
// "mints once" because the value is deterministic, not because it tracks state.
// The id is validated for path/shell safety (it flows into a persisted filename
// and a CR name).
func mintOperationID(version string, markerCreatedAt time.Time) (string, error) {
	id := fmt.Sprintf("upgrade-%s-%s", version, markerCreatedAt.UTC().Format(sanity.TimestampLayout))
	if err := sanity.ValidateOperationID(id); err != nil {
		return "", errx.WithHints(
			errx.WithReason(errManifestInvalid.Wrap(err, "derived operationId %q is invalid", id), ReasonManifestInvalid),
			"Confirm images.consensusNode.version is a clean version string (no spaces or shell metacharacters)",
		)
	}
	return id, nil
}

// prepareCRName builds the RFC 1123-safe metadata.name for the Prepare CR:
// "<operationId-lowercased>-prepare". metadata.name carries no semantic meaning
// (operationId in the spec/label is the authoritative identity); it embeds the
// operationId only to stay unique across sequential upgrades, since
// NetworkUpgrade* CRs are never deleted.
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
