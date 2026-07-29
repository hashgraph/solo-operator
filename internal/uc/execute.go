// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"strings"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// executeCRName builds the metadata.name for the Execute CR:
// "<operationId-lowercased>-execute". Per-node uniqueness comes from the
// operationId itself (node0-upgrade-…), so sibling nodes never collide despite
// one CR per ConsensusCapsule. Deterministic in the operationId, so a re-detected
// marker is a no-op (Create → AlreadyExists).
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
