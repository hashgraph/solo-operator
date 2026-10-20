// SPDX-License-Identifier: Apache-2.0

package common

import corev1 "k8s.io/api/core/v1"

// RestrictedSecurityContext blocks privilege escalation and drops all capabilities.
func RestrictedSecurityContext() *corev1.SecurityContext {
	allowPrivilegeEscalation := false
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}
