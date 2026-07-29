// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// kindNetworkUpgradeFreeze is the CR kind the freeze_scheduled.mf handler creates.
const kindNetworkUpgradeFreeze = "NetworkUpgradeFreeze"

// freezeCRName builds the RFC 1123-safe metadata.name for the Freeze CR:
// "<operationId-lowercased>-freeze" — e.g. upgrade-v0.75.0-20260413t120000z-freeze.
// Identity lives in spec.operationId, not the name; the name embeds the
// operationId only so sequential upgrades never collide.
func freezeCRName(operationID string) string {
	return strings.ToLower(operationID) + "-" + operatorv1alpha1.StepFreeze
}

// parseFreezeScheduledTime reads the epoch-seconds timestamp written by the
// consensus node into freeze_scheduled.mf and converts it to UTC time.Time.
// The CN writes the scheduled freeze time as decimal seconds since Unix epoch.
func parseFreezeScheduledTime(upgradePath string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Join(upgradePath, MarkerFreezeScheduled))
	if err != nil {
		return time.Time{}, errx.WithReason(errPersist.Wrap(err, "read %s", MarkerFreezeScheduled), ReasonMarkerStatFailed)
	}
	epochStr := strings.TrimSpace(string(data))
	epoch, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		return time.Time{}, errx.WithReason(
			errManifestInvalid.Wrap(err, "parse epoch timestamp %q in %s", epochStr, MarkerFreezeScheduled),
			ReasonManifestInvalid)
	}
	return time.Unix(epoch, 0).UTC(), nil
}

// buildFreezeCR constructs the NetworkUpgradeFreeze CR as an unstructured object
// ready for persist-before-send and the dynamic-client Create.
//
// spec.images and spec.selectedSources are copied from the corresponding
// NetworkUpgradePrepare CR, making the Freeze CR self-contained. If the Prepare
// CR's status.selectedSources is nil (PrepareReconciler has not yet run), an
// empty SelectedSources is carried forward — the FreezeReconciler can re-derive
// it from spec.images at reconcile time (HIP-1496 FreezeReconciler step 3).
func buildFreezeCR(
	cfg Config,
	operationID string,
	scheduledTime time.Time,
	images operatorv1alpha1.UpgradeImagesSpec,
	selectedSources operatorv1alpha1.SelectedSources,
) (*unstructured.Unstructured, error) {
	freeze := &operatorv1alpha1.NetworkUpgradeFreeze{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1alpha1.GroupVersion.String(),
			Kind:       kindNetworkUpgradeFreeze,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      freezeCRName(operationID),
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID: operationID,
				operatorv1alpha1.LabelStep:        operatorv1alpha1.StepFreeze,
			},
			Annotations: map[string]string{
				annotationCreatedBy: createdByUC,
			},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
			Orbit:           cfg.Orbit,
			OperationID:     operationID,
			ScheduledTime:   metav1.NewTime(scheduledTime),
			Images:          images,
			SelectedSources: selectedSources,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(freeze)
	if err != nil {
		return nil, errx.WithReason(
			errCRBuild.Wrap(err, "convert NetworkUpgradeFreeze %s to unstructured", operationID), ReasonCRBuildFailed)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}
