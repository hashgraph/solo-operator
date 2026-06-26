// SPDX-License-Identifier: Apache-2.0

package simplefeesschedules

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfSimpleFeesSchedules = "simple-fees-schedules"
	ConfigMapSuffix           = "-simple-fee-schedules-cm"
	ConfigFileName            = "simpleFeesSchedules.json"
)

func CommonLabels(obj *operatorv1alpha1.SimpleFeesSchedules) map[string]string {
	return map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfSimpleFeesSchedules,
		"app":                 obj.Name,
	}
}

func ConfigMapName(obj *operatorv1alpha1.SimpleFeesSchedules) string {
	return obj.Name + ConfigMapSuffix
}

func ConfigMapNameForRef(name string) string {
	return name + ConfigMapSuffix
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.SimpleFeesSchedules] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme, recorder),
	)
}
