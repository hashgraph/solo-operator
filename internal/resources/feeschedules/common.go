// SPDX-License-Identifier: Apache-2.0

package feeschedules

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfFeeSchedules = "fee-schedules"
	ConfigFileName     = "feeSchedules.json"
	ConfigMapSuffix    = "fee-schedules"
)

func CommonLabels(obj *operatorv1alpha1.FeeSchedules) map[string]string {
	labels := map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfFeeSchedules,
		"app":                 obj.Name,
	}
	common.PropagateOperationID(labels, obj)
	return labels
}

func ConfigMapName(obj *operatorv1alpha1.FeeSchedules) string {
	return obj.Spec.Scope + "-" + ConfigMapSuffix
}

func ConfigMapNameForRef(name string) string {
	return name
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.FeeSchedules] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme, recorder),
	)
}
