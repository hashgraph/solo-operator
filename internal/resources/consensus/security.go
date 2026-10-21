// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/hashgraph/solo-operator/internal/common"
)

// hardenedSecurityContext fills fields unset in override from the restricted baseline.
func hardenedSecurityContext(override *corev1.SecurityContext) *corev1.SecurityContext {
	baseline := common.RestrictedSecurityContext()
	if override == nil {
		return baseline
	}
	sc := override.DeepCopy()
	if sc.Capabilities == nil {
		sc.Capabilities = &corev1.Capabilities{}
	}
	if sc.Capabilities.Drop == nil {
		sc.Capabilities.Drop = baseline.Capabilities.Drop
	}
	// API server rejects allowPrivilegeEscalation=false with privileged or CAP_SYS_ADMIN.
	if sc.AllowPrivilegeEscalation == nil && !grantsPrivilege(sc) {
		sc.AllowPrivilegeEscalation = baseline.AllowPrivilegeEscalation
	}
	if sc.RunAsNonRoot == nil && sc.RunAsUser != nil && *sc.RunAsUser == 0 {
		runAsNonRoot := false
		sc.RunAsNonRoot = &runAsNonRoot
	}
	return sc
}

func grantsPrivilege(sc *corev1.SecurityContext) bool {
	if sc.Privileged != nil && *sc.Privileged {
		return true
	}
	for _, c := range sc.Capabilities.Add {
		if c == "SYS_ADMIN" || c == "CAP_SYS_ADMIN" {
			return true
		}
	}
	return false
}

func runAsSecurityContext(uid, gid int64) *corev1.SecurityContext {
	sc := common.RestrictedSecurityContext()
	sc.RunAsUser = &uid
	sc.RunAsGroup = &gid
	return sc
}

func hardenedPodSecurityContext(override *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	psc := &corev1.PodSecurityContext{}
	if override != nil {
		psc = override.DeepCopy()
	}
	if psc.SeccompProfile == nil {
		psc.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	}
	if psc.RunAsNonRoot == nil && (psc.RunAsUser == nil || *psc.RunAsUser != 0) {
		runAsNonRoot := true
		psc.RunAsNonRoot = &runAsNonRoot
	}
	return psc
}
