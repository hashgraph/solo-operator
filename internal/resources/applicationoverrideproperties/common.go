// SPDX-License-Identifier: Apache-2.0

package applicationoverrideproperties

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfApplicationOverrideProperties = "application-override-properties"
	ConfigMapSuffix                     = "-application-override-properties-cm"
	ConfigFileName                      = "application-override.properties"
)

func CommonLabels(obj *operatorv1alpha1.ApplicationOverrideProperties) map[string]string {
	return map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfApplicationOverrideProperties,
		"app":                 obj.Name,
	}
}

func ConfigMapName(obj *operatorv1alpha1.ApplicationOverrideProperties) string {
	return obj.Name + ConfigMapSuffix
}

func ConfigMapNameForRef(name string) string {
	return name + ConfigMapSuffix
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.ApplicationOverrideProperties] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme, recorder),
	)
}
