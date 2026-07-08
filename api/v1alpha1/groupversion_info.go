// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the operator v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.solo.hedera.com
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion = schema.GroupVersion{Group: "operator.solo.hedera.com", Version: "v1alpha1"}

	//nolint:staticcheck // scheme.Builder is the standard kubebuilder pattern for api packages
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)
