// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	PartOfLog4j2Config = "log4j2-config"
	ConfigFileName     = "log4j2.xml"
	ConfigMapSuffix    = "log4j2"
)

func CommonLabels(obj *operatorv1alpha1.Log4j2Config) map[string]string {
	labels := map[string]string{
		common.LabelManagedBy: common.ManagedByValue,
		common.LabelPartOf:    PartOfLog4j2Config,
		"app":                 obj.Name,
	}
	common.PropagateOperationID(labels, obj)
	return labels
}

func ConfigMapName(obj *operatorv1alpha1.Log4j2Config) string {
	return obj.Spec.Scope + "-" + ConfigMapSuffix
}

// ConfigMapNameForRef returns the ConfigMap name for a Log4j2Config referenced by name.
func ConfigMapNameForRef(name string) string {
	return name
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.Log4j2Config] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme, recorder),
	)
}
