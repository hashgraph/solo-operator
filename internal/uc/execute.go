// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// executeCRName builds the metadata.name for the Execute CR:
// "<operationId-lowercased>-execute" — e.g. upgrade-0.75.0-20260413t120000z-execute.
// mintOperationID validates that this name — operationId lowercased plus the
// longest step suffix — is a valid DNS-1123 subdomain, so the result is always a
// valid RFC 1123 name.
//
// Identity lives in spec.operationId (+ the operation-id label), not the name. The
// name is a deterministic function of the operationId: distinct upgrades get
// distinct names (never colliding) and the same upgrade always maps to the same
// name — so a re-detected marker is a no-op (Create returns AlreadyExists), and a
// CR deleted then recreated (e.g. during DR, even mid-upgrade) comes back under the
// same identity rather than as a second, parallel CR. NetworkUpgrade* CRs are
// retained after completion, but the naming does not depend on them being immortal.
func executeCRName(operationID string) string {
	return strings.ToLower(operationID) + "-" + operatorv1alpha1.StepExecute
}

// buildExecuteCR constructs the NetworkUpgradeExecute CR as an unstructured object
// ready for persist-before-send and the dynamic-client Create.
//
// spec.images and spec.selectedSources are copied verbatim from the
// NetworkUpgradeFreeze CR, which in turn carried them verbatim from the Prepare
// CR — so the Execute CR is self-contained and pins the registry selection made
// once at prepare (the operator's ExecuteReconciler re-validates it into
// status.selectedSources). spec.consensusNode scopes the CR to this UC's owning
// ConsensusCapsule (HIP-1496 "NetworkUpgradeExecute" — one CR per capsule); the
// operator resolves the ConsensusCapsule by that name. Config.Validate guarantees
// it is non-empty.
func buildExecuteCR(
	cfg Config,
	operationID string,
	images operatorv1alpha1.UpgradeImagesSpec,
	selectedSources operatorv1alpha1.SelectedSources,
) (*unstructured.Unstructured, error) {
	execute := &operatorv1alpha1.NetworkUpgradeExecute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1alpha1.GroupVersion.String(),
			Kind:       kindNetworkUpgradeExecute,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      executeCRName(operationID),
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID: operationID,
				operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
			},
			Annotations: map[string]string{
				annotationCreatedBy: createdByUC,
			},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:           cfg.Orbit,
			OperationID:     operationID,
			ConsensusNode:   cfg.CapsuleName(),
			Images:          images,
			SelectedSources: selectedSources,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(execute)
	if err != nil {
		return nil, errx.WithReason(
			errCRBuild.Wrap(err, "convert NetworkUpgradeExecute %s to unstructured", operationID), ReasonCRBuildFailed)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}
