// SPDX-License-Identifier: Apache-2.0

// Package helmtest provides a mock helm.Client for use in tests.
package helmtest

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
)

// MockClient is an in-memory helm.Client for tests.
type MockClient struct {
	releases map[string]*helm.ReleaseInfo

	InstallOrUpgradeFunc func(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts helm.ReleaseOptions) (*helm.ReleaseInfo, error)
	UninstallFunc        func(ctx context.Context, releaseName, namespace string, opts helm.UninstallOptions) error
	GetReleaseFunc       func(releaseName, namespace string) (*helm.ReleaseInfo, error)
}

func NewMockClient() *MockClient {
	return &MockClient{releases: make(map[string]*helm.ReleaseInfo)}
}

func (m *MockClient) InstallOrUpgrade(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts helm.ReleaseOptions) (*helm.ReleaseInfo, error) {
	if m.InstallOrUpgradeFunc != nil {
		return m.InstallOrUpgradeFunc(ctx, releaseName, namespace, chart, values, opts)
	}
	key := namespace + "/" + releaseName
	existing := m.releases[key]
	version := 1
	if existing != nil {
		version = existing.Version + 1
	}
	info := &helm.ReleaseInfo{
		Name:        releaseName,
		Namespace:   namespace,
		Version:     version,
		Status:      "deployed",
		ChartString: chart.Chart + "-" + chart.Version,
	}
	m.releases[key] = info
	return info, nil
}

func (m *MockClient) Uninstall(ctx context.Context, releaseName, namespace string, opts helm.UninstallOptions) error {
	if m.UninstallFunc != nil {
		return m.UninstallFunc(ctx, releaseName, namespace, opts)
	}
	delete(m.releases, namespace+"/"+releaseName)
	return nil
}

func (m *MockClient) GetRelease(releaseName, namespace string) (*helm.ReleaseInfo, error) {
	if m.GetReleaseFunc != nil {
		return m.GetReleaseFunc(releaseName, namespace)
	}
	info, ok := m.releases[namespace+"/"+releaseName]
	if !ok {
		return nil, nil
	}
	return info, nil
}
