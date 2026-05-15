// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

const (
	LabelManagedBy = common.LabelManagedBy
	LabelOrbit     = common.LabelOrbit
	ManagedByValue = common.ManagedByValue
)

func StandardLabels(orbit *operatorv1alpha1.Orbit) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelOrbit:     orbit.Name,
	}
}
