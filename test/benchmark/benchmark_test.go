// SPDX-License-Identifier: Apache-2.0

package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/resources/envoy"
	"github.com/hashgraph/solo-operator/internal/resources/haproxy"
	"github.com/hashgraph/solo-operator/internal/resources/orbit"
	"github.com/hashgraph/solo-operator/internal/sequencer"
)

func init() {
	_ = operatorv1alpha1.AddToScheme(scheme.Scheme)
}

// =============================================================================
// Sequencer Benchmarks
// =============================================================================

func BenchmarkSequencerComputeDelay(b *testing.B) {
	// Create a pool of operations for the supplier
	operations := make([]*operatorv1alpha1.NetworkOperation, 50)
	for i := 0; i < 50; i++ {
		operations[i] = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("op-%d", i),
				Namespace:         "test-ns",
				UID:               types.UID(fmt.Sprintf("uid-%d", i)),
				CreationTimestamp: metav1.Now(),
			},
		}
	}

	seq := sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
		return operations
	})

	// Refresh to populate cache
	seq.Refresh()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := sequencer.NewRequest(operations[i%50])
		seq.ComputeReconcileDelay(request)
	}
}

func BenchmarkSequencerCheckNext(b *testing.B) {
	// Create operations across multiple namespaces
	operations := make([]*operatorv1alpha1.NetworkOperation, 100)
	for i := 0; i < 100; i++ {
		operations[i] = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("op-%d", i),
				Namespace:         fmt.Sprintf("ns-%d", i%10),
				UID:               types.UID(fmt.Sprintf("uid-%d", i)),
				CreationTimestamp: metav1.Now(),
			},
		}
	}

	seq := sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
		return operations
	})

	seq.Refresh()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := sequencer.NewRequest(operations[i%100])
		seq.CheckNext(request)
	}
}

func BenchmarkSequencerRefresh(b *testing.B) {
	operations := make([]*operatorv1alpha1.NetworkOperation, 100)
	for i := 0; i < 100; i++ {
		operations[i] = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("op-%d", i),
				Namespace:         fmt.Sprintf("ns-%d", i%10),
				UID:               types.UID(fmt.Sprintf("uid-%d", i)),
				CreationTimestamp: metav1.Now(),
			},
		}
	}

	seq := sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
		return operations
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq.Refresh()
	}
}

func BenchmarkSequencerMarkAsCompleted(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create fresh sequencer for each iteration
		op := &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "op-0",
				Namespace:         "test-ns",
				UID:               types.UID("uid-0"),
				CreationTimestamp: metav1.Now(),
			},
		}
		operations := []*operatorv1alpha1.NetworkOperation{op}
		seq := sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
			return operations
		})
		seq.Refresh()
		// Mark as executing first (required before completion)
		seq.CheckNext(sequencer.NewRequest(op))
		b.StartTimer()

		request := sequencer.NewRequest(op)
		seq.MarkAsCompleted(request)
	}
}

// =============================================================================
// Subreconciler Benchmarks
// =============================================================================

func BenchmarkOrbitSubReconcilers(b *testing.B) {
	ctx := context.Background()

	orbitObj := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bench-orbit",
		},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace:       "bench-orbit",
			CreateNamespace: true,
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings:          map[string]string{"ledger.id": "0x03"},
				ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
				PermissionConfiguration:  map[string]string{"permission": "default"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(orbitObj).
		Build()

	chain := orbit.NewSubReconcilers(fakeClient, scheme.Scheme)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chain.Reconcile(ctx, orbitObj)
	}
}

func BenchmarkConsensusCapsuleSubReconcilers(b *testing.B) {
	ctx := context.Background()

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
			Log4j2XML: "<Configuration></Configuration>",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Name: "consensus-node",
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "hashgraph",
							ImageName:  "hedera-node",
							ImageTag:   "v0.50.0",
						},
					},
				},
			},
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(capsule, ns).
		Build()

	chain := consensus.NewSubReconcilers(fakeClient, scheme.Scheme)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chain.Reconcile(ctx, capsule)
	}
}

func BenchmarkHAProxySubReconcilers(b *testing.B) {
	ctx := context.Background()

	haproxyObj := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-haproxy",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			Image:       "haproxy:2.8.10-alpine3.20",
			ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(haproxyObj, ns).
		Build()

	chain := haproxy.NewSubReconcilers(fakeClient, scheme.Scheme)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chain.Reconcile(ctx, haproxyObj)
	}
}

func BenchmarkEnvoySubReconcilers(b *testing.B) {
	ctx := context.Background()

	replicas := int32(2)
	envoyObj := &operatorv1alpha1.EnvoyProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-envoy",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.EnvoyProxySpec{
			Image:     "envoyproxy/envoy-distroless:v1.21.6",
			Replicas:  &replicas,
			ProxyPort: 8080,
			AdminPort: 9901,
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(envoyObj, ns).
		Build()

	chain := envoy.NewSubReconcilers(fakeClient, scheme.Scheme)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chain.Reconcile(ctx, envoyObj)
	}
}

// =============================================================================
// Memory Benchmarks
// =============================================================================

func BenchmarkMemoryOrbitCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("orbit-%d", i),
			},
			Spec: operatorv1alpha1.OrbitSpec{
				Namespace: fmt.Sprintf("ns-%d", i),
				Lifetime:  "24h",
				Consensus: operatorv1alpha1.OrbitConsensus{
					NetworkSettings:          map[string]string{"ledger.id": "0x03", "hedera.realm": "0"},
					ApplicationConfiguration: map[string]string{"hedera.realm": "0", "hedera.shard": "0"},
					PermissionConfiguration:  map[string]string{"permission": "default"},
				},
			},
		}
	}
}

func BenchmarkMemoryConsensusCapsuleCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("node%d", i),
				Namespace: "test-ns",
			},
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				Orbit:     "test-orbit",
				NodeId:    int64(i),
				AccountId: fmt.Sprintf("0.0.%d", i+3),
				Weight:    1,
				Log4j2XML: "<Configuration></Configuration>",
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
							Name: "consensus-node",
							SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
								Repository: "hashgraph",
								ImageName:  "hedera-node",
								ImageTag:   "v0.50.0",
							},
						},
					},
				},
			},
		}
	}
}

func BenchmarkMemoryNetworkOperationCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("op-%d", i),
				Namespace: "test-ns",
			},
			Spec: operatorv1alpha1.NetworkOperationSpec{
				Orbit: "test-orbit",
				Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
			},
		}
	}
}

// =============================================================================
// Concurrent Access Benchmarks
// =============================================================================

func BenchmarkSequencerConcurrentAccess(b *testing.B) {
	operations := make([]*operatorv1alpha1.NetworkOperation, 100)
	for i := 0; i < 100; i++ {
		operations[i] = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("op-%d", i),
				Namespace:         fmt.Sprintf("ns-%d", i%10),
				UID:               types.UID(fmt.Sprintf("uid-%d", i)),
				CreationTimestamp: metav1.Now(),
			},
		}
	}

	seq := sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
		return operations
	})
	seq.Refresh()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			request := sequencer.NewRequest(operations[i%100])
			seq.CheckNext(request)
			seq.ComputeReconcileDelay(request)
			i++
		}
	})
}

// =============================================================================
// Client Operations Benchmarks
// =============================================================================

func BenchmarkFakeClientCreate(b *testing.B) {
	ctx := context.Background()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("orbit-%d", i),
			},
			Spec: operatorv1alpha1.OrbitSpec{
				Namespace: fmt.Sprintf("ns-%d", i),
				Consensus: operatorv1alpha1.OrbitConsensus{
					NetworkSettings:          map[string]string{"ledger.id": "0x03"},
					ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
					PermissionConfiguration:  map[string]string{"permission": "default"},
				},
			},
		}
		_ = fakeClient.Create(ctx, obj)
	}
}

func BenchmarkFakeClientGet(b *testing.B) {
	ctx := context.Background()

	orbitObj := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test-orbit",
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings:          map[string]string{"ledger.id": "0x03"},
				ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
				PermissionConfiguration:  map[string]string{"permission": "default"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(orbitObj).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &operatorv1alpha1.Orbit{}
		_ = fakeClient.Get(ctx, client.ObjectKey{Name: "test-orbit"}, result)
	}
}

func BenchmarkFakeClientList(b *testing.B) {
	ctx := context.Background()

	objects := make([]client.Object, 100)
	for i := 0; i < 100; i++ {
		objects[i] = &operatorv1alpha1.NetworkOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("op-%d", i),
				Namespace: "test-ns",
			},
			Spec: operatorv1alpha1.NetworkOperationSpec{
				Orbit: "test-orbit",
				Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
			},
		}
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(objects...).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := &operatorv1alpha1.NetworkOperationList{}
		_ = fakeClient.List(ctx, list, client.InNamespace("test-ns"))
	}
}

// =============================================================================
// Serialization Benchmarks
// =============================================================================

func BenchmarkOrbitDeepCopy(b *testing.B) {
	orbitObj := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
			Labels: map[string]string{
				"app": "test",
				"env": "bench",
			},
			Annotations: map[string]string{
				"description": "benchmark test orbit",
			},
		},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test-orbit",
			Lifetime:  "24h",
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings: map[string]string{
					"ledger.id":    "0x03",
					"hedera.realm": "0",
					"hedera.shard": "0",
				},
				ApplicationConfiguration: map[string]string{
					"hedera.realm": "0",
					"hedera.shard": "0",
				},
				PermissionConfiguration: map[string]string{
					"permission": "default",
				},
			},
		},
		Status: operatorv1alpha1.OrbitStatus{
			Phase: operatorv1alpha1.PhaseRunning,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = orbitObj.DeepCopy()
	}
}

func BenchmarkConsensusCapsuleDeepCopy(b *testing.B) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app":    "consensus",
				"nodeId": "0",
			},
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
			Log4j2XML: "<Configuration></Configuration>",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Name: "consensus-node",
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "hashgraph",
							ImageName:  "hedera-node",
							ImageTag:   "v0.50.0",
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = capsule.DeepCopy()
	}
}

// =============================================================================
// GVK Operations Benchmarks
// =============================================================================

func BenchmarkGVKLookup(b *testing.B) {
	gvk := schema.GroupVersionKind{
		Group:   "operator.solo.hedera.com",
		Version: "v1alpha1",
		Kind:    "Orbit",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = scheme.Scheme.New(gvk)
	}
}

// =============================================================================
// Test Runner for Manual Benchmarking with Detailed Output
// =============================================================================

func TestPrintBenchmarkSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark summary in short mode")
	}

	fmt.Println("\n============================================================")
	fmt.Println("Solo Operator Performance Benchmark Summary")
	fmt.Println("============================================================")
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("GOOS/GOARCH: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("NumCPU: %d\n", runtime.NumCPU())

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("\nInitial Memory Stats:\n")
	fmt.Printf("  Alloc: %d MB\n", m.Alloc/1024/1024)
	fmt.Printf("  TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
	fmt.Printf("  Sys: %d MB\n", m.Sys/1024/1024)
	fmt.Printf("  NumGC: %d\n", m.NumGC)

	fmt.Println("\nTo run full benchmarks:")
	fmt.Println("  go test -bench=. -benchmem ./test/benchmark/...")
	fmt.Println("  go test -bench=BenchmarkSequencer -benchmem ./test/benchmark/...")
	fmt.Println("  go test -bench=BenchmarkMemory -benchmem ./test/benchmark/...")
	fmt.Println("============================================================")
}
