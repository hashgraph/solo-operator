// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfApplicationProperties = "application-properties"
	ConfigMapSuffix             = "-appprops"
	ConfigFileName              = "application.properties"
)

func CommonLabels(obj *operatorv1alpha1.ApplicationProperties) map[string]string {
	return map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfApplicationProperties,
		"app":                 obj.Name,
	}
}

func ConfigMapName(obj *operatorv1alpha1.ApplicationProperties) string {
	return obj.Name + ConfigMapSuffix
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.ApplicationProperties] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
	)
}
