// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// MockClient is an in-memory Client for use in helm-package tests only.
// External test packages should use internal/resources/helmtest instead.
type MockClient struct {
	releases map[string]*ReleaseInfo

	InstallOrUpgradeFunc func(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error)
	UninstallFunc        func(ctx context.Context, releaseName, namespace string, opts UninstallOptions) error
	GetReleaseFunc       func(releaseName, namespace string) (*ReleaseInfo, error)
}

func NewMockClient() *MockClient {
	return &MockClient{releases: make(map[string]*ReleaseInfo)}
}

func (m *MockClient) InstallOrUpgrade(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
	if m.InstallOrUpgradeFunc != nil {
		return m.InstallOrUpgradeFunc(ctx, releaseName, namespace, chart, values, opts)
	}
	key := namespace + "/" + releaseName
	existing := m.releases[key]
	version := 1
	if existing != nil {
		version = existing.Version + 1
	}
	info := &ReleaseInfo{
		Name:        releaseName,
		Namespace:   namespace,
		Version:     version,
		Status:      "deployed",
		ChartString: chart.Chart + "-" + chart.Version,
	}
	m.releases[key] = info
	return info, nil
}

func (m *MockClient) Uninstall(ctx context.Context, releaseName, namespace string, opts UninstallOptions) error {
	if m.UninstallFunc != nil {
		return m.UninstallFunc(ctx, releaseName, namespace, opts)
	}
	delete(m.releases, namespace+"/"+releaseName)
	return nil
}

func (m *MockClient) GetRelease(releaseName, namespace string) (*ReleaseInfo, error) {
	if m.GetReleaseFunc != nil {
		return m.GetReleaseFunc(releaseName, namespace)
	}
	info, ok := m.releases[namespace+"/"+releaseName]
	if !ok {
		return nil, nil
	}
	return info, nil
}
