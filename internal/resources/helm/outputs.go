// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"fmt"
	"strconv"
	"strings"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	defaultBlockNodePort int32 = 40840
	maxResourceNameLen   int   = 63
)

func componentOutputs(hc *operatorv1alpha1.HelmCapsule, releaseName string, values map[string]interface{}) map[string]string {
	if hc.Spec.Component == nil {
		return nil
	}
	switch *hc.Spec.Component {
	case operatorv1alpha1.HelmComponentBlockNode:
		return blockNodeOutputs(hc, releaseName, values)
	default:
		return nil
	}
}

func blockNodeOutputs(hc *operatorv1alpha1.HelmCapsule, releaseName string, values map[string]interface{}) map[string]string {
	servicePort := portValue(nestedValue(values, "service", "port"), defaultBlockNodePort)
	return map[string]string{
		"address":       fmt.Sprintf("%s.%s.svc.cluster.local", chartFullname(releaseName, hc.Spec.Chart.Chart, values), hc.Namespace),
		"streamingPort": strconv.Itoa(int(portValue(nestedValue(values, "blockNode", "ports", "publisher"), servicePort))),
		"servicePort":   strconv.Itoa(int(portValue(nestedValue(values, "blockNode", "ports", "serverStatus"), servicePort))),
	}
}

// chartFullname mirrors the chart's fullname helper, which names the Service.
func chartFullname(releaseName, chartName string, values map[string]interface{}) string {
	if override, _ := nestedValue(values, "fullnameOverride").(string); override != "" {
		return trimResourceName(override)
	}
	name := chartName
	if override, _ := nestedValue(values, "nameOverride").(string); override != "" {
		name = override
	}
	if strings.Contains(releaseName, name) {
		return trimResourceName(releaseName)
	}
	return trimResourceName(releaseName + "-" + name)
}

func trimResourceName(name string) string {
	if len(name) > maxResourceNameLen {
		name = name[:maxResourceNameLen]
	}
	return strings.TrimSuffix(name, "-")
}

func nestedValue(values map[string]interface{}, path ...string) interface{} {
	var current interface{} = values
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func portValue(v interface{}, fallback int32) int32 {
	var n int64
	switch p := v.(type) {
	case float64:
		n = int64(p)
	case int:
		n = int64(p)
	case int32:
		n = int64(p)
	case int64:
		n = p
	case string:
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return fallback
		}
		n = parsed
	}
	if n < 1 || n > 65535 {
		return fallback
	}
	return int32(n)
}
