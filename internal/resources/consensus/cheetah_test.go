// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func TestCheetahConfigMapSubReconciler_Reconcile(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	tests := []struct {
		name            string
		capsule         *operatorv1alpha1.ConsensusCapsule
		expectConfigMap bool
		expectFiles     []string
	}{
		{
			name: "no uploaders enabled",
			capsule: &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test-ns",
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									ImageName: "test",
									ImageTag:  "latest",
								},
							},
						},
					},
				},
			},
			expectConfigMap: false,
		},
		{
			name: "blocks uploader enabled",
			capsule: &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test-ns",
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					AccountId: "0.0.3",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									ImageName: "test",
									ImageTag:  "latest",
								},
							},
							BlocksUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Debug",
								GCS: &operatorv1alpha1.CheetahGCSConfig{
									Enabled: true,
									Bucket:  "test-bucket",
								},
							},
						},
					},
				},
			},
			expectConfigMap: true,
			expectFiles:     []string{"cheetah-block-streams.yaml"},
		},
		{
			name: "all uploaders enabled",
			capsule: &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test-ns",
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					AccountId: "0.0.3",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									ImageName: "test",
									ImageTag:  "latest",
								},
							},
							BlocksUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Info",
							},
							RecordsUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Debug",
							},
							EventsUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Info",
							},
						},
					},
				},
			},
			expectConfigMap: true,
			expectFiles:     []string{"cheetah-block-streams.yaml", "cheetah-record-streams.yaml", "cheetah-events-streams.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(scheme, tt.capsule)
			reconciler := NewCheetahConfigMapSubReconciler(c, scheme)

			result, err := reconciler.Reconcile(ctx, tt.capsule)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Requeue {
				t.Error("unexpected requeue")
			}

			// Check if ConfigMap was created
			cm := &corev1.ConfigMap{}
			err = c.Get(ctx, client.ObjectKey{Name: CheetahConfigMapName, Namespace: tt.capsule.Namespace}, cm)

			if tt.expectConfigMap {
				if err != nil {
					t.Fatalf("expected ConfigMap to be created, got error: %v", err)
				}

				// Check expected files exist
				for _, file := range tt.expectFiles {
					if _, ok := cm.Data[file]; !ok {
						t.Errorf("expected file %s not found in ConfigMap", file)
					}
				}
			} else if err == nil {
				t.Error("expected no ConfigMap, but one was created")
			}
		})
	}
}

func TestCheetahConfigMapSubReconciler_BuildConfigMap(t *testing.T) {
	scheme := newTestScheme()
	reconciler := NewCheetahConfigMapSubReconciler(nil, scheme)

	tests := []struct {
		name        string
		capsule     *operatorv1alpha1.ConsensusCapsule
		expectFiles []string
	}{
		{
			name: "blocks only",
			capsule: &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test-ns",
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					AccountId: "0.0.3",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									ImageName: "test",
									ImageTag:  "latest",
								},
							},
							BlocksUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Debug",
							},
						},
					},
				},
			},
			expectFiles: []string{"cheetah-block-streams.yaml"},
		},
		{
			name: "all uploaders",
			capsule: &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test-ns",
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					AccountId: "0.0.3",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									ImageName: "test",
									ImageTag:  "latest",
								},
							},
							BlocksUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Info",
							},
							RecordsUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Debug",
							},
							EventsUploader: &operatorv1alpha1.CheetahUploader{
								Enabled:  true,
								LogLevel: "Info",
							},
						},
					},
				},
			},
			expectFiles: []string{"cheetah-block-streams.yaml", "cheetah-record-streams.yaml", "cheetah-events-streams.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := reconciler.buildConfigMap(tt.capsule, tt.capsule.Spec.PodProperties.Containers.RecordsUploader, tt.capsule.Spec.PodProperties.Containers.EventsUploader, tt.capsule.Spec.PodProperties.Containers.BlocksUploader)

			// Check expected files exist
			for _, file := range tt.expectFiles {
				if _, ok := cm.Data[file]; !ok {
					t.Errorf("expected file %s not found in ConfigMap", file)
				}
			}
		})
	}
}

func TestBuildCheetahUploaderContainer(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			AccountId: "0.0.3",
		},
	}

	tests := []struct {
		name             string
		uploader         *operatorv1alpha1.CheetahUploader
		containerName    string
		configFile       string
		mountPath        string
		envPrefix        string
		streamBucketPath string
		expectEnvVars    []string
		expectPrefixVal  string
	}{
		{
			name: "blocks uploader with GCS",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				Name:    "blockstream-uploader",
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
					Repository: "ghcr.io/hashgraph",
					ImageName:  "cheetah",
					ImageTag:   "0.3.1",
				},
				GCS: &operatorv1alpha1.CheetahGCSConfig{
					Enabled:  true,
					Bucket:   "test-bucket",
					Region:   "us-east-1",
					Endpoint: "https://storage.googleapis.com",
					UseSsl:   true,
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					},
				},
				EnvFromSecret: "test-secret",
			},
			containerName:    "blockstream-uploader",
			configFile:       "cheetah-block-streams.yaml",
			mountPath:        "/opt/hgcapp/blockStreams",
			envPrefix:        "BLOCK_STREAMS",
			streamBucketPath: "blockStreams/block-0.0.3",
			expectEnvVars:    []string{"GCS_ENABLED", "GCS_BUCKET_NAME", "GCS_BUCKET_REGION", "GCS_ENDPOINT", "GCS_USE_SSL", "GCS_BLOCK_STREAMS_BUCKET_PREFIX"},
			expectPrefixVal:  "blockStreams/block-0.0.3",
		},
		{
			name: "records uploader with S3",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
					Repository: "ghcr.io/hashgraph",
					ImageName:  "cheetah",
					ImageTag:   "latest",
				},
				S3: &operatorv1alpha1.CheetahS3Config{
					Enabled:  true,
					Bucket:   "s3-bucket",
					Region:   "us-west-2",
					Endpoint: "s3.amazonaws.com",
					UseSsl:   true,
				},
			},
			containerName:    "record-stream-uploader",
			configFile:       "cheetah-record-streams.yaml",
			mountPath:        "/opt/hgcapp/recordStreams",
			envPrefix:        "RECORD_STREAMS",
			streamBucketPath: "recordstreams/record0.0.3",
			expectEnvVars:    []string{"S3_ENABLED", "S3_BUCKET_NAME", "S3_BUCKET_REGION", "S3_ENDPOINT", "S3_USE_SSL", "S3_RECORD_STREAMS_BUCKET_PREFIX"},
			expectPrefixVal:  "recordstreams/record0.0.3",
		},
		{
			name: "blocks uploader with BucketPrefix",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
					Repository: "ghcr.io/hashgraph",
					ImageName:  "cheetah",
					ImageTag:   "0.3.1",
				},
				BucketPrefix: "mynetwork/testnet",
				S3: &operatorv1alpha1.CheetahS3Config{
					Enabled: true,
					Bucket:  "my-bucket",
				},
			},
			containerName:    "blockstream-uploader",
			configFile:       "cheetah-block-streams.yaml",
			mountPath:        "/opt/hgcapp/blockStreams",
			envPrefix:        "BLOCK_STREAMS",
			streamBucketPath: "blockStreams/block-0.0.3",
			expectEnvVars:    []string{"S3_ENABLED", "S3_BLOCK_STREAMS_BUCKET_PREFIX"},
			expectPrefixVal:  "mynetwork/testnet/blockStreams/block-0.0.3",
		},
		{
			name: "blocks uploader with custom ConfigCheckInterval",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
					Repository: "ghcr.io/hashgraph",
					ImageName:  "cheetah",
					ImageTag:   "0.4.5",
				},
				ConfigCheckInterval: "60s",
				S3: &operatorv1alpha1.CheetahS3Config{
					Enabled: true,
					Bucket:  "my-bucket",
				},
			},
			containerName:    "blockstream-uploader",
			configFile:       "cheetah-block-streams.yaml",
			mountPath:        "/opt/hgcapp/blockStreams",
			envPrefix:        "BLOCK_STREAMS",
			streamBucketPath: "blockStreams/block-0.0.3",
			expectEnvVars:    []string{"S3_ENABLED", "S3_BLOCK_STREAMS_BUCKET_PREFIX"},
			expectPrefixVal:  "blockStreams/block-0.0.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := reconciler.buildCheetahUploaderContainer(capsule, tt.uploader, tt.containerName, tt.configFile, tt.mountPath, tt.envPrefix, tt.streamBucketPath)

			// Check container name
			if container.Name != tt.containerName {
				t.Errorf("expected container name %s, got %s", tt.containerName, container.Name)
			}

			// Check image
			expectedImage := common.BuildImageOrDefault(tt.uploader.SoftwareVersion, CheetahDefaultImage)
			if container.Image != expectedImage {
				t.Errorf("expected image %s, got %s", expectedImage, container.Image)
			}

			// Check command includes --config-check-interval
			expectedInterval := "30s"
			if tt.uploader.ConfigCheckInterval != "" {
				expectedInterval = tt.uploader.ConfigCheckInterval
			}
			expectedCmd := []string{"/app/bin/cheetah", "upload", "--config", "/app/config/cheetah.yaml", "--config-check-interval", expectedInterval}
			if len(container.Command) != len(expectedCmd) {
				t.Errorf("expected command length %d, got %d", len(expectedCmd), len(container.Command))
			}
			for i, arg := range expectedCmd {
				if i < len(container.Command) && container.Command[i] != arg {
					t.Errorf("command[%d]: expected %q, got %q", i, arg, container.Command[i])
				}
			}

			// Check container ports
			if len(container.Ports) != 2 {
				t.Errorf("expected 2 ports, got %d", len(container.Ports))
			} else {
				portMap := make(map[string]int32)
				for _, p := range container.Ports {
					portMap[p.Name] = p.ContainerPort
				}
				if portMap["pprof"] != 6061 {
					t.Errorf("expected pprof port 6061, got %d", portMap["pprof"])
				}
				if portMap["stats"] != 6060 {
					t.Errorf("expected stats port 6060, got %d", portMap["stats"])
				}
			}

			// Check environment variables exist and prefix value is correct
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}
			for _, expectedEnv := range tt.expectEnvVars {
				if _, ok := envMap[expectedEnv]; !ok {
					t.Errorf("expected environment variable %s not found", expectedEnv)
				}
			}
			// Verify bucket prefix value for whichever storage backend is configured
			prefixKey := fmt.Sprintf("S3_%s_BUCKET_PREFIX", tt.envPrefix)
			if tt.uploader.GCS != nil {
				prefixKey = fmt.Sprintf("GCS_%s_BUCKET_PREFIX", tt.envPrefix)
			}
			if got, ok := envMap[prefixKey]; ok {
				if got != tt.expectPrefixVal {
					t.Errorf("bucket prefix: expected %q, got %q", tt.expectPrefixVal, got)
				}
			}

			// Check volume mounts
			if len(container.VolumeMounts) != 2 {
				t.Errorf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
			}

			// No per-container security context — fsGroup at pod level handles ownership
			if container.SecurityContext != nil {
				t.Error("cheetah container should not set a per-container security context")
			}

			// Check resources
			if tt.uploader.Resources != nil {
				if container.Resources.Limits.Cpu().Cmp(*tt.uploader.Resources.Limits.Cpu()) != 0 {
					t.Error("resource limits do not match")
				}
			}

			// Check envFrom
			if tt.uploader.EnvFromSecret != "" {
				if len(container.EnvFrom) != 1 {
					t.Errorf("expected 1 envFrom, got %d", len(container.EnvFrom))
				}
				if container.EnvFrom[0].SecretRef.Name != tt.uploader.EnvFromSecret {
					t.Errorf("expected secret %s, got %s", tt.uploader.EnvFromSecret, container.EnvFrom[0].SecretRef.Name)
				}
			}

			// Check config-check-interval in command
			if tt.uploader.ConfigCheckInterval != "" {
				if len(container.Command) >= 6 && container.Command[5] != tt.uploader.ConfigCheckInterval {
					t.Errorf("expected config-check-interval %q, got %q", tt.uploader.ConfigCheckInterval, container.Command[5])
				}
			} else {
				// default should be 30s
				if len(container.Command) >= 6 && container.Command[5] != "30s" {
					t.Errorf("expected default config-check-interval 30s, got %q", container.Command[5])
				}
			}
		})
	}
}

func TestGetVolumeNameForPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/opt/hgcapp/blockStreams", VolumeTypeBlocks},
		{"/opt/hgcapp/recordStreams", VolumeTypeRecords},
		{"/opt/hgcapp/eventsStreams", VolumeTypeEvents},
		{"/unknown/path", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := getVolumeNameForPath(tt.path)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestAddSidecarContainers_Cheetah(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			AccountId: "0.0.3",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							ImageName: "test",
							ImageTag:  "latest",
						},
					},
					BlocksUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "ghcr.io/hashgraph",
							ImageName:  "cheetah",
							ImageTag:   "0.3.1",
						},
					},
					RecordsUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "ghcr.io/hashgraph",
							ImageName:  "cheetah",
							ImageTag:   "0.3.1",
						},
					},
					EventsUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "ghcr.io/hashgraph",
							ImageName:  "cheetah",
							ImageTag:   "0.3.1",
						},
					},
				},
			},
		},
	}

	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	reconciler.addSidecarContainers(deploy, capsule)

	// Should have 4 containers: consensus-node + 3 cheetah uploaders
	if len(deploy.Spec.Template.Spec.Containers) != 4 {
		t.Errorf("expected 4 containers, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	// Check container names
	expectedNames := []string{"consensus-node", "blockstream-uploader", "record-stream-uploader", "event-stream-uploader"}
	for i, expected := range expectedNames {
		if i < len(deploy.Spec.Template.Spec.Containers) {
			if deploy.Spec.Template.Spec.Containers[i].Name != expected {
				t.Errorf("expected container %d to be %s, got %s", i, expected, deploy.Spec.Template.Spec.Containers[i].Name)
			}
		}
	}

	// Check cheetah config volume was added
	foundConfigVolume := false
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == "solo-cheetah-config" {
			foundConfigVolume = true
			break
		}
	}
	if !foundConfigVolume {
		t.Error("expected solo-cheetah-config volume to be added")
	}

	// No init container — fsGroup handles ownership instead
	if len(deploy.Spec.Template.Spec.InitContainers) != 0 {
		t.Errorf("expected no init containers, got %d", len(deploy.Spec.Template.Spec.InitContainers))
	}
}

func TestApplyCheetahFSGroup(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}

	t.Run("sets fsGroup 2000 when cheetah enabled and no security context set", func(t *testing.T) {
		capsule := &operatorv1alpha1.ConsensusCapsule{
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						BlocksUploader: &operatorv1alpha1.CheetahUploader{Enabled: true},
					},
				},
			},
		}
		deploy := &appsv1.Deployment{}
		reconciler.applyCheetahFSGroup(deploy, capsule)
		if deploy.Spec.Template.Spec.SecurityContext == nil || deploy.Spec.Template.Spec.SecurityContext.FSGroup == nil {
			t.Fatal("expected fsGroup to be set")
		}
		if *deploy.Spec.Template.Spec.SecurityContext.FSGroup != 2000 {
			t.Errorf("expected fsGroup 2000, got %d", *deploy.Spec.Template.Spec.SecurityContext.FSGroup)
		}
	})

	t.Run("does not override user-provided fsGroup", func(t *testing.T) {
		capsule := &operatorv1alpha1.ConsensusCapsule{
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						BlocksUploader: &operatorv1alpha1.CheetahUploader{Enabled: true},
					},
				},
			},
		}
		userFSGroup := int64(1000)
		deploy := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{FSGroup: &userFSGroup},
					},
				},
			},
		}
		reconciler.applyCheetahFSGroup(deploy, capsule)
		if *deploy.Spec.Template.Spec.SecurityContext.FSGroup != 1000 {
			t.Errorf("expected user fsGroup 1000 to be preserved, got %d", *deploy.Spec.Template.Spec.SecurityContext.FSGroup)
		}
	})

	t.Run("no-op when cheetah not enabled", func(t *testing.T) {
		capsule := &operatorv1alpha1.ConsensusCapsule{}
		deploy := &appsv1.Deployment{}
		reconciler.applyCheetahFSGroup(deploy, capsule)
		if deploy.Spec.Template.Spec.SecurityContext != nil {
			t.Error("expected no security context when cheetah not enabled")
		}
	})
}

func TestCheetahBucketPath(t *testing.T) {
	tests := []struct {
		bucketPrefix string
		streamPath   string
		expected     string
	}{
		{"", "blockStreams/block-0.0.3", "blockStreams/block-0.0.3"},
		{"mynetwork/testnet", "blockStreams/block-0.0.3", "mynetwork/testnet/blockStreams/block-0.0.3"},
		{"prefix", "recordstreams/record0.0.3", "prefix/recordstreams/record0.0.3"},
	}
	for _, tt := range tests {
		got := cheetahBucketPath(tt.bucketPrefix, tt.streamPath)
		if got != tt.expected {
			t.Errorf("cheetahBucketPath(%q, %q) = %q, want %q", tt.bucketPrefix, tt.streamPath, got, tt.expected)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
