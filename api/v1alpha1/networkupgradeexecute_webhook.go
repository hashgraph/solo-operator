// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-operator-solo-hedera-com-v1alpha1-networkupgradeexecute,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=update;delete,versions=v1alpha1,name=vnetworkupgradeexecute.kb.io,admissionReviewVersions=v1

type NetworkUpgradeExecuteValidator struct{}

func (v *NetworkUpgradeExecuteValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return builder.WebhookManagedBy[*NetworkUpgradeExecute](mgr, &NetworkUpgradeExecute{}).
		WithValidator(v).
		Complete()
}

func (v *NetworkUpgradeExecuteValidator) ValidateCreate(_ context.Context, _ *NetworkUpgradeExecute) (admission.Warnings, error) {
	return nil, nil
}

func (v *NetworkUpgradeExecuteValidator) ValidateUpdate(_ context.Context, old, _ *NetworkUpgradeExecute) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeExecute %q is immutable after creation", old.Name)
}

func (v *NetworkUpgradeExecuteValidator) ValidateDelete(_ context.Context, obj *NetworkUpgradeExecute) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeExecute %q cannot be deleted", obj.Name)
}
