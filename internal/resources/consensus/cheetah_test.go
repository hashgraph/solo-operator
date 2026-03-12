// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestCheetahConfigMapSubReconciler_Reconcile(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	tests := []struct {
		name           string
		capsule        *operatorv1alpha1.ConsensusCapsule
		expectConfigMap bool
		expectFiles    []string
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
								Image: "test:latest",
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
								Image: "test:latest",
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
								Image: "test:latest",
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
			} else {
				if err == nil {
					t.Error("expected no ConfigMap, but one was created")
				}
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
								Image: "test:latest",
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
								Image: "test:latest",
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
		name          string
		uploader      *operatorv1alpha1.CheetahUploader
		containerName string
		configFile    string
		mountPath     string
		envPrefix     string
		expectEnvVars []string
	}{
		{
			name: "blocks uploader with GCS",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				Name:    "blockstream-uploader",
				Image:   "cheetah:0.3.1",
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
			containerName: "blockstream-uploader",
			configFile:    "cheetah-block-streams.yaml",
			mountPath:     "/opt/hgcapp/blockStreams",
			envPrefix:     "BLOCK_STREAMS",
			expectEnvVars: []string{"GCS_ENABLED", "GCS_BUCKET_NAME", "GCS_BUCKET_REGION", "GCS_ENDPOINT", "GCS_USE_SSL", "GCS_BLOCK_STREAMS_BUCKET_PREFIX"},
		},
		{
			name: "records uploader with S3",
			uploader: &operatorv1alpha1.CheetahUploader{
				Enabled: true,
				Image:   "cheetah:latest",
				S3: &operatorv1alpha1.CheetahS3Config{
					Enabled:  true,
					Bucket:   "s3-bucket",
					Region:   "us-west-2",
					Endpoint: "s3.amazonaws.com",
					UseSsl:   true,
				},
			},
			containerName: "record-stream-uploader",
			configFile:    "cheetah-record-streams.yaml",
			mountPath:     "/opt/hgcapp/recordStreams",
			envPrefix:     "RECORD_STREAMS",
			expectEnvVars: []string{"S3_ENABLED", "S3_BUCKET_NAME", "S3_BUCKET_REGION", "S3_ENDPOINT", "S3_USE_SSL", "S3_RECORD_STREAMS_BUCKET_PREFIX"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := reconciler.buildCheetahUploaderContainer(capsule, tt.uploader, tt.containerName, tt.configFile, tt.mountPath, tt.envPrefix)

			// Check container name
			if container.Name != tt.containerName {
				t.Errorf("expected container name %s, got %s", tt.containerName, container.Name)
			}

			// Check image
			if container.Image != tt.uploader.Image {
				t.Errorf("expected image %s, got %s", tt.uploader.Image, container.Image)
			}

			// Check command
			expectedCmd := []string{"/app/bin/cheetah", "upload", "--config", "/app/config/cheetah.yaml"}
			if len(container.Command) != len(expectedCmd) {
				t.Errorf("expected command length %d, got %d", len(expectedCmd), len(container.Command))
			}

			// Check environment variables
			envMap := make(map[string]bool)
			for _, env := range container.Env {
				envMap[env.Name] = true
			}
			for _, expectedEnv := range tt.expectEnvVars {
				if !envMap[expectedEnv] {
					t.Errorf("expected environment variable %s not found", expectedEnv)
				}
			}

			// Check volume mounts
			if len(container.VolumeMounts) != 2 {
				t.Errorf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
			}

			// Check security context
			if container.SecurityContext == nil {
				t.Error("expected security context to be set")
			} else {
				if *container.SecurityContext.RunAsUser != 2000 {
					t.Errorf("expected runAsUser 2000, got %d", *container.SecurityContext.RunAsUser)
				}
				if *container.SecurityContext.RunAsGroup != 2000 {
					t.Errorf("expected runAsGroup 2000, got %d", *container.SecurityContext.RunAsGroup)
				}
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
		})
	}
}

func TestGetVolumeNameForPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/opt/hgcapp/blockStreams", VolumeTypeBlockStreams},
		{"/opt/hgcapp/recordStreams", VolumeTypeRecordStreams},
		{"/opt/hgcapp/eventsStreams", VolumeTypeEventsStreams},
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
						Image: "test:latest",
					},
					BlocksUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						Image:   "cheetah:0.3.1",
					},
					RecordsUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						Image:   "cheetah:0.3.1",
					},
					EventsUploader: &operatorv1alpha1.CheetahUploader{
						Enabled: true,
						Image:   "cheetah:0.3.1",
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
