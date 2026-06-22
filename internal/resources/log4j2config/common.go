// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfLog4j2Config = "log4j2-config"
	ConfigMapSuffix    = "-log4j2"
	ConfigFileName     = "log4j2.xml"
)

func CommonLabels(obj *operatorv1alpha1.Log4j2Config) map[string]string {
	return map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfLog4j2Config,
		"app":                 obj.Name,
	}
}

func ConfigMapName(obj *operatorv1alpha1.Log4j2Config) string {
	return obj.Name + ConfigMapSuffix
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.Log4j2Config] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
	)
}
