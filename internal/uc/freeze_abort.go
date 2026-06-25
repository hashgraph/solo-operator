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

// kindNetworkUpgradeFreezeAbort is the CR kind the freeze_aborted.mf handler
// creates.
const kindNetworkUpgradeFreezeAbort = "NetworkUpgradeFreezeAbort"

// abortCRName builds the RFC 1123-safe metadata.name for the FreezeAbort CR:
// "<operationId-lowercased>-abort" — e.g. upgrade-v0.75.0-20260413t120000z-abort.
// The operationId is the prepare-anchored identity (it embeds the package
// version + prepare-marker time, never the freeze time); the name embeds it only
// so sequential upgrades never collide, since NetworkUpgrade* CRs are never
// deleted. Identity lives in the operation-id label / spec, not the name.
func abortCRName(operationID string) string {
	return strings.ToLower(operationID) + "-" + operatorv1alpha1.StepAbort
}

// buildFreezeAbortCR constructs the NetworkUpgradeFreezeAbort CR as an
// unstructured object ready for persist-before-send and the dynamic-client
// Create. The UC supplies only the identity (orbit + operationId); the operator's
// FreezeAbortReconciler owns all logic, including the AbortAfterFreeze safety
// guard when an Execute CR already exists for the operationId. Prior Prepare /
// Freeze CRs are deliberately left in place as an audit trail.
func buildFreezeAbortCR(cfg Config, operationID string) (*unstructured.Unstructured, error) {
	abort := &operatorv1alpha1.NetworkUpgradeFreezeAbort{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1alpha1.GroupVersion.String(),
			Kind:       kindNetworkUpgradeFreezeAbort,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      abortCRName(operationID),
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID: operationID,
				operatorv1alpha1.LabelStep:        operatorv1alpha1.StepAbort,
			},
			Annotations: map[string]string{
				annotationCreatedBy: createdByUC,
			},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeAbortSpec{
			Orbit:       cfg.Orbit,
			OperationID: operationID,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(abort)
	if err != nil {
		return nil, errx.WithReason(
			errCRBuild.Wrap(err, "convert NetworkUpgradeFreezeAbort %s to unstructured", operationID), "CRBuildFailed")
	}
	return &unstructured.Unstructured{Object: obj}, nil
}
