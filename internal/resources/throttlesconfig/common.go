// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfThrottlesConfig = "throttles-config"
	ConfigFileName        = "throttles.json"
	ConfigMapSuffix       = "throttles"
)

func CommonLabels(obj *operatorv1alpha1.ThrottlesConfig) map[string]string {
	return map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfThrottlesConfig,
		"app":                 obj.Name,
	}
}

func ConfigMapName(obj *operatorv1alpha1.ThrottlesConfig) string {
	return obj.Spec.Scope + "-" + ConfigMapSuffix
}

func ConfigMapNameForRef(name string) string {
	return name
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.ThrottlesConfig] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme, recorder),
	)
}
