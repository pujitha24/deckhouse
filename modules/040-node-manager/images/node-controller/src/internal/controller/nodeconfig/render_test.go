/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// TestKeepBootstrapOnlyFields covers what happens to the first master on its
// first day-2 render. Its spec came from a dhctl payload, and only the fields
// below have no answer anywhere in the cluster, so only they survive the
// wholesale patch.
//
// Registry and serverTLSBootstrap deliberately do NOT: both are rendered from
// the cluster now. Carrying them over made the master keep whatever the
// bootstrap chose forever — a self-signed kubelet serving certificate with no
// IP in it (so `kubectl exec` and `kubectl logs` failed against every pod on the
// master), and, on the create path where there is no existing object to carry
// anything from, no registry at all.
func TestKeepBootstrapOnlyFields(t *testing.T) {
	bootstrapped := internalv1alpha1.NodeSpec{
		Registry: &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce"},
		Storage: internalv1alpha1.Storage{
			Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
		},
		Kubelet: internalv1alpha1.Kubelet{
			ServerTLSBootstrap:  ptr.To(false),
			NodeIP:              "10.0.0.10",
			ResourceReservation: &internalv1alpha1.ResourceReservation{Mode: "Auto"},
		},
	}

	tests := []struct {
		name       string
		existing   internalv1alpha1.NodeSpec
		expStorage internalv1alpha1.Storage
		expNodeIP  string
	}{
		{
			name:       "bootstrapped master keeps only what the cluster cannot render",
			existing:   bootstrapped,
			expStorage: bootstrapped.Storage,
			expNodeIP:  "10.0.0.10",
		},
		{
			name:       "worker with nothing to keep is left as rendered",
			existing:   internalv1alpha1.NodeSpec{Storage: renderStorage()},
			expStorage: renderStorage(),
		},
		{
			name: "an explicit device is as specific as a selector",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
			},
			expStorage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered := &internalv1alpha1.Registry{Address: "rendered.example.com"}
			desired := internalv1alpha1.NodeSpec{
				NodeName: "master-0",
				Registry: rendered,
				Storage:  renderStorage(),
				Kubelet:  internalv1alpha1.Kubelet{MaxPods: defaultMaxPods},
			}

			keepBootstrapOnlyFields(&desired, &tc.existing)

			require.Equal(t, tc.expStorage, desired.Storage)
			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
			require.Equal(t, tc.existing.Kubelet.ResourceReservation, desired.Kubelet.ResourceReservation)

			// The cluster owns these two, so the render wins even when the node
			// was bootstrapped with something else.
			require.Equal(t, rendered, desired.Registry)
			require.Nil(t, desired.Kubelet.ServerTLSBootstrap)

			// Everything else still comes from the render.
			require.Equal(t, "master-0", desired.NodeName)
			require.Equal(t, defaultMaxPods, desired.Kubelet.MaxPods)
		})
	}
}

// kubelet exits when --node-labels carries a label it may not accept, so a node
// whose group template names one comes up with no kubelet at all. The master
// group names two of them.
func TestRenderNodeLabelsDropsWhatKubeletRefuses(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudPermanent,
			NodeTemplate: &v1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
					"node.deckhouse.io/group":               "master",
					"topology.kubernetes.io/zone":           "eu-1a",
					"node.kubernetes.io/instance-type":      "m1",
					"example.com/team":                      "candi",
				},
			},
		},
	}

	labels := renderNodeLabels(ng)

	for _, refused := range []string{"node-role.kubernetes.io/control-plane", "node-role.kubernetes.io/master"} {
		_, present := labels[refused]
		require.False(t, present, "%s must not reach --node-labels", refused)
	}
	for _, kept := range []string{"node.deckhouse.io/group", "topology.kubernetes.io/zone", "node.kubernetes.io/instance-type", "example.com/team"} {
		require.Contains(t, labels, kept)
	}
}

// The tuning a node needs to survive load, not just to boot. Every one of these
// is absent from a stock kernel in a way that shows up only under pressure —
// pods failing on inotify limits, packet loss from an overflowed ARP table —
// when the cluster gets blamed instead of the missing setting.
func TestRenderKernelCarriesTheTuningANodeNeeds(t *testing.T) {
	sysctl := renderKernel().Sysctl

	required := map[string]string{
		"fs.inotify.max_user_instances":     "5120",
		"fs.inotify.max_user_watches":       "524288",
		"kernel.pid_max":                    "2000000",
		"net.ipv4.neigh.default.gc_thresh3": "32768",
		"net.core.somaxconn":                "1000",
		"vm.overcommit_memory":              "1",
		"vm.min_free_kbytes":                "131072",
		// cilium needs reverse-path filtering off, and "default" is what covers
		// the pod interfaces that do not exist yet.
		"net.ipv4.conf.all.rp_filter":     "0",
		"net.ipv4.conf.default.rp_filter": "0",
		// kubelet refuses to start without these.
		"kernel.panic":         "10",
		"kernel.panic_on_oops": "1",
	}
	for key, want := range required {
		require.Equal(t, internalv1alpha1.SysctlValue(want), sysctl[key], "sysctl %s", key)
	}

	// Derived from the node's core count, so the cluster must not guess it.
	require.NotContains(t, sysctl, "net.netfilter.nf_conntrack_max")
}

// Settings a NodeGroup accepts but the agent's schema does not. Passing them
// through renders a NodeConfig the API server rejects, and a node whose config
// is rejected never receives one — so a setting meant to tune the node takes it
// out of the cluster instead.
func TestRenderSurvivesSettingsTheAgentSchemaRejects(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:    v1.NodeTypeCloudEphemeral,
			Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeRollingUpdate},
			Kubelet:     &v1.KubeletSpec{MaxPods: ptr.To(int32(1000))},
		},
	}

	// RollingUpdate has no counterpart on the node; the agent knows Manual and
	// Automatic only.
	require.Equal(t, string(v1.DisruptionApprovalModeAutomatic), renderUpdatePolicy(ng).Mode)

	// The documentation suggests 1000 for a /21 pod subnet; the schema stops at
	// 500. A ceiling is a better answer than no config at all.
	require.Equal(t, maxPodsCeiling, renderKubelet(ng, &corev1.Node{}, clusterInputs{}).MaxPods)
}

// kubelet compares label namespaces by suffix. Equality here dropped legal
// labels silently, and let through two that do not exist upstream at all —
// which kubelet refuses, taking the node down with it.
func TestKubeletMaySetLabelFollowsKubeletsOwnRule(t *testing.T) {
	allowed := []string{
		"node.deckhouse.io/group",
		"example.com/team",
		"topology.kubernetes.io/zone",
		"node.kubernetes.io/instance-type",
		// Under an allowed prefix: kubelet accepts it, so must we.
		"myprefix.node.kubernetes.io/foo",
		"myprefix.kubelet.kubernetes.io/bar",
	}
	for _, key := range allowed {
		require.True(t, kubeletMaySetLabel(key), "kubelet accepts %s", key)
	}

	refused := []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		"kubernetes.io/anything-else",
		// No GA spelling of these exists; kubelet exits on them.
		"failure-domain.kubernetes.io/zone",
		"failure-domain.kubernetes.io/region",
	}
	for _, key := range refused {
		require.False(t, kubeletMaySetLabel(key), "kubelet refuses %s", key)
	}
}
