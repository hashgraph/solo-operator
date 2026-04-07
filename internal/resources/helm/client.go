// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	DefaultTimeout    = 5 * time.Minute
	helmStorageDriver = "secrets"
)

var (
	ErrChartFetch          = errors.New("chart fetch failed")
	ErrValuesBuild         = errors.New("values build failed")
	ErrReleaseNameConflict = errors.New("release name conflict")
)

type ReleaseInfo struct {
	Name        string
	Namespace   string
	Version     int
	Status      string
	ChartString string // "chartname-version"
}

// ChartCredentials is read from the Secret named in HelmChartSpec.CredentialsRef.
type ChartCredentials struct {
	Username string
	Password string
}

// ReleaseOptions maps to HelmCapsuleSpec fields. Zero values match Helm defaults.
type ReleaseOptions struct {
	Timeout         time.Duration
	Wait            bool
	Atomic          bool
	CleanupOnFail   bool
	MaxHistory      int
	CreateNamespace bool
	DisableHooks    bool
	// Credentials are used to authenticate to a private chart repository.
	// Nil means no authentication (public repo).
	Credentials *ChartCredentials
}

type UninstallOptions struct {
	Timeout      time.Duration // defaults to DefaultTimeout when zero
	KeepHistory  bool          // helm uninstall --keep-history
	DisableHooks bool
	Wait         bool
}

// Client abstracts Helm operations for testability.
type Client interface {
	InstallOrUpgrade(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error)
	Uninstall(ctx context.Context, releaseName, namespace string, opts UninstallOptions) error
	GetRelease(releaseName, namespace string) (*ReleaseInfo, error)
}

// ─── Real implementation ──────────────────────────────────────────────────────

type helmSDKClient struct {
	restConfig *rest.Config
}

// NewHelmSDKClient returns a Client backed by helm.sh/helm/v3. Pass mgr.GetConfig().
func NewHelmSDKClient(restConfig *rest.Config) Client {
	return &helmSDKClient{restConfig: restConfig}
}

func (h *helmSDKClient) actionConfig(namespace string) (*action.Configuration, error) {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("creating helm registry client: %w", err)
	}

	cfg := &action.Configuration{RegistryClient: registryClient}
	getter := &restConfigGetter{restConfig: h.restConfig, namespace: namespace}

	if err := cfg.Init(getter, namespace, helmStorageDriver, func(string, ...interface{}) {}); err != nil {
		return nil, fmt.Errorf("initialising helm action config: %w", err)
	}
	return cfg, nil
}

func (h *helmSDKClient) InstallOrUpgrade(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
	existing, err := h.GetRelease(releaseName, namespace)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return h.install(ctx, releaseName, namespace, chart, values, opts)
	}
	return h.upgrade(ctx, releaseName, namespace, chart, values, opts)
}

// loginIfNeeded logs in to the OCI registry when credentials are set. No-op for plain HTTPS repos.
func loginIfNeeded(registryClient *registry.Client, chart operatorv1alpha1.HelmChartSpec, creds *ChartCredentials) error {
	if creds == nil || !isOCI(chart.Repo) {
		return nil
	}
	host := ociHost(chart.Repo)
	return registryClient.Login(host, registry.LoginOptBasicAuth(creds.Username, creds.Password))
}

// ociHost extracts the registry host from an oci:// URL.
// "oci://ghcr.io/hashgraph/solo-charts" → "ghcr.io"
func ociHost(repo string) string {
	without := strings.TrimPrefix(repo, "oci://")
	if idx := strings.IndexByte(without, '/'); idx != -1 {
		return without[:idx]
	}
	return without
}

func (h *helmSDKClient) install(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	if err := loginIfNeeded(cfg.RegistryClient, chart, opts.Credentials); err != nil {
		return nil, fmt.Errorf("authenticating to chart repository: %w", err)
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = opts.CreateNamespace
	install.Timeout = opts.Timeout
	install.Wait = opts.Wait
	install.Atomic = opts.Atomic
	install.DisableHooks = opts.DisableHooks
	install.Version = chart.Version

	ref, err := chartRef(chart)
	if err != nil {
		return nil, err
	}
	chartPath, err := install.LocateChart(ref, cli.New())
	if err != nil {
		return nil, fmt.Errorf("%w: locating chart %s: %w", ErrChartFetch, ref, err)
	}
	c, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("%w: loading chart %s: %w", ErrChartFetch, chartPath, err)
	}
	rel, err := install.RunWithContext(ctx, c, values)
	if err != nil {
		return nil, fmt.Errorf("helm install %s: %w", releaseName, err)
	}
	return releaseInfoFrom(rel), nil
}

func (h *helmSDKClient) upgrade(ctx context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	if err := loginIfNeeded(cfg.RegistryClient, chart, opts.Credentials); err != nil {
		return nil, fmt.Errorf("authenticating to chart repository: %w", err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Timeout = opts.Timeout
	upgrade.Wait = opts.Wait
	upgrade.Atomic = opts.Atomic
	upgrade.CleanupOnFail = opts.CleanupOnFail
	upgrade.MaxHistory = opts.MaxHistory
	upgrade.DisableHooks = opts.DisableHooks
	upgrade.ReuseValues = false
	upgrade.Version = chart.Version

	ref, err := chartRef(chart)
	if err != nil {
		return nil, err
	}
	chartPath, err := upgrade.LocateChart(ref, cli.New())
	if err != nil {
		return nil, fmt.Errorf("%w: locating chart %s: %w", ErrChartFetch, ref, err)
	}
	c, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("%w: loading chart %s: %w", ErrChartFetch, chartPath, err)
	}
	rel, err := upgrade.RunWithContext(ctx, releaseName, c, values)
	if err != nil {
		return nil, fmt.Errorf("helm upgrade %s: %w", releaseName, err)
	}
	return releaseInfoFrom(rel), nil
}

func (h *helmSDKClient) Uninstall(_ context.Context, releaseName, namespace string, opts UninstallOptions) error {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return err
	}
	uninstall := action.NewUninstall(cfg)
	if opts.Timeout > 0 {
		uninstall.Timeout = opts.Timeout
	} else {
		uninstall.Timeout = DefaultTimeout
	}
	uninstall.KeepHistory = opts.KeepHistory
	uninstall.DisableHooks = opts.DisableHooks
	uninstall.Wait = opts.Wait
	if _, err := uninstall.Run(releaseName); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("helm uninstall %s: %w", releaseName, err)
	}
	return nil
}

func (h *helmSDKClient) GetRelease(releaseName, namespace string) (*ReleaseInfo, error) {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	rel, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("helm get %s: %w", releaseName, err)
	}
	return releaseInfoFrom(rel), nil
}

func releaseInfoFrom(rel *helmrelease.Release) *ReleaseInfo {
	if rel == nil {
		return nil
	}
	info := &ReleaseInfo{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Version:   rel.Version,
	}
	if rel.Info != nil {
		info.Status = rel.Info.Status.String()
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		info.ChartString = rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version
	}
	return info
}

func isNotFound(err error) bool {
	return errors.Is(err, driver.ErrReleaseNotFound)
}

// chartRef builds the ref passed to LocateChart. OCI gets a full oci:// ref;
// HTTP gets a direct .tgz URL so we don't need `helm repo add`.
func chartRef(chart operatorv1alpha1.HelmChartSpec) (string, error) {
	if isOCI(chart.Repo) {
		return fmt.Sprintf("%s/%s", strings.TrimRight(chart.Repo, "/"), chart.Chart), nil
	}
	if chart.Version == "" {
		return "", fmt.Errorf("%w: chart version is required for HTTP repos", ErrChartFetch)
	}
	return fmt.Sprintf("%s/%s-%s.tgz", strings.TrimRight(chart.Repo, "/"), chart.Chart, chart.Version), nil
}

func isOCI(repo string) bool {
	return strings.HasPrefix(repo, "oci://")
}

// ─── Mock implementation ──────────────────────────────────────────────────────

// MockClient is an in-memory Client for tests.
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

// ─── RESTClientGetter ─────────────────────────────────────────────────────────

// restConfigGetter lets Helm use the same cluster connection as controller-runtime.
type restConfigGetter struct {
	restConfig *rest.Config
	namespace  string
}

func (r *restConfigGetter) ToRESTConfig() (*rest.Config, error) {
	return rest.CopyConfig(r.restConfig), nil
}

func (r *restConfigGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(r.restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (r *restConfigGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(r.restConfig)
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc)), nil
}

func (r *restConfigGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["default"] = &clientcmdapi.Cluster{
		Server:                   r.restConfig.Host,
		CertificateAuthorityData: r.restConfig.CAData,
		CertificateAuthority:     r.restConfig.CAFile,
		InsecureSkipTLSVerify:    r.restConfig.Insecure,
	}
	cfg.AuthInfos["default"] = &clientcmdapi.AuthInfo{
		Token:                 r.restConfig.BearerToken,
		TokenFile:             r.restConfig.BearerTokenFile,
		ClientCertificateData: r.restConfig.CertData,
		ClientKeyData:         r.restConfig.KeyData,
		ClientCertificate:     r.restConfig.CertFile,
		ClientKey:             r.restConfig.KeyFile,
	}
	cfg.Contexts["default"] = &clientcmdapi.Context{
		Cluster:   "default",
		AuthInfo:  "default",
		Namespace: r.namespace,
	}
	cfg.CurrentContext = "default"
	return clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
}
