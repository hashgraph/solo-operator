// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-operator-solo-hedera-com-v1alpha1-networkupgradefreezeabort,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts,verbs=update;delete,versions=v1alpha1,name=vnetworkupgradefreezeabort.kb.io,admissionReviewVersions=v1

type NetworkUpgradeFreezeAbortValidator struct{}

func (v *NetworkUpgradeFreezeAbortValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return builder.WebhookManagedBy[*NetworkUpgradeFreezeAbort](mgr, &NetworkUpgradeFreezeAbort{}).
		WithValidator(v).
		Complete()
}

func (v *NetworkUpgradeFreezeAbortValidator) ValidateCreate(_ context.Context, _ *NetworkUpgradeFreezeAbort) (admission.Warnings, error) {
	return nil, nil
}

func (v *NetworkUpgradeFreezeAbortValidator) ValidateUpdate(_ context.Context, old, _ *NetworkUpgradeFreezeAbort) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeFreezeAbort %q is immutable after creation", old.Name)
}

func (v *NetworkUpgradeFreezeAbortValidator) ValidateDelete(_ context.Context, obj *NetworkUpgradeFreezeAbort) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeFreezeAbort %q cannot be deleted", obj.Name)
}
