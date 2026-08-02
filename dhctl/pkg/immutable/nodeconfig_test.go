// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

const (
	testContainerdDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testCNIDigest        = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testKubeletDigest    = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testPauseDigest      = "sha256:4444444444444444444444444444444444444444444444444444444444444444"

	testEtcdDigest              = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	testAPIServerDigest         = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	testControllerManagerDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	testSchedulerDigest         = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
)

func testMetaConfig(t *testing.T, rootDiskSize, etcdDiskSize string) *config.MetaConfig {
	t.Helper()

	masterNodeGroup := fmt.Sprintf(`{
	  "replicas": 1,
	  "instanceClass": {
	    "rootDisk": {"size": %q},
	    "etcdDisk": {"size": %q}
	  }
	}`, rootDiskSize, etcdDiskSize)

	metaConfig := &config.MetaConfig{
		ClusterType:       config.CloudClusterType,
		ClusterPrefix:     "zykov",
		ClusterDomain:     "cluster.local",
		ClusterDNSAddress: "10.223.0.10",
		ClusterConfig: map[string]json.RawMessage{
			"kubernetesVersion":       json.RawMessage(`"1.34"`),
			"serviceSubnetCIDR":       json.RawMessage(`"10.223.0.0/16"`),
			"podSubnetCIDR":           json.RawMessage(`"10.222.0.0/16"`),
			"podSubnetNodeCIDRPrefix": json.RawMessage(`"24"`),
			"clusterDomain":           json.RawMessage(`"cluster.local"`),
		},
		ProviderClusterConfig: map[string]json.RawMessage{
			"masterNodeGroup": json.RawMessage(masterNodeGroup),
		},
		Images: map[string]map[string]any{
			"registrypackages": {
				"containerdSysext224":    testContainerdDigest,
				"kubernetesCniSysext162": testCNIDigest,
				"kubeletSysext1349":      testKubeletDigest,
				"kubeletSysext1336":      "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			},
			"common": {
				"pause": testPauseDigest,
			},
			"controlPlaneManager": {
				"etcd":                     testEtcdDigest,
				"kubeApiserver134":         testAPIServerDigest,
				"kubeControllerManager134": testControllerManagerDigest,
				"kubeScheduler134":         testSchedulerDigest,
			},
		},
	}

	metaConfig.Registry.Settings = registry.ModeSettings{
		Mode: constant.ModeUnmanaged,
		RemoteData: registry.Data{
			ImagesRepo: "dev-registry.deckhouse.io/sys/deckhouse-oss",
			Scheme:     constant.SchemeHTTPS,
			Username:   "user",
			Password:   "password",
		},
	}

	return metaConfig
}

// Neither disk is selected by the installer: the node tells them apart by what
// is on them. Sizes take no part, so the pair that used to be rejected — two
// disks of one size — is now as valid as any other.
func TestMasterDisksNameNothing(t *testing.T) {
	for _, sizes := range []struct{ root, etcd string }{
		{"50Gi", "10Gi"},
		{"20Gi", "20Gi"},
		{"10Gi", "20Gi"},
	} {
		t.Run(sizes.root+"/"+sizes.etcd, func(t *testing.T) {
			systemDisk, controlPlaneDisk, err := MasterDisks(testMetaConfig(t, sizes.root, sizes.etcd))
			require.NoError(t, err)
			require.Equal(t, Disk{}, systemDisk)
			require.Equal(t, Disk{}, controlPlaneDisk)
		})
	}
}

func TestMasterDisksMissingEtcdDisk(t *testing.T) {
	metaConfig := testMetaConfig(t, "50Gi", "10Gi")
	metaConfig.ProviderClusterConfig["masterNodeGroup"] = json.RawMessage(`{"instanceClass": {"rootDisk": {"size": "50Gi"}}}`)

	_, _, err := MasterDisks(metaConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), "etcdDisk.size is not set")
}

func TestSysextDigestsMissing(t *testing.T) {
	metaConfig := testMetaConfig(t, "50Gi", "10Gi")
	delete(metaConfig.Images["registrypackages"], "kubeletSysext1349")

	_, err := SysextDigests(metaConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), `no "kubelet" system extension digest for Kubernetes 1.34`)
}

func TestSysextDigestsPicksNewestPatch(t *testing.T) {
	metaConfig := testMetaConfig(t, "50Gi", "10Gi")
	metaConfig.Images["registrypackages"]["kubeletSysext13410"] = testKubeletDigest
	metaConfig.Images["registrypackages"]["kubeletSysext1349"] = "sha256:9999999999999999999999999999999999999999999999999999999999999999"

	digests, err := SysextDigests(metaConfig)
	require.NoError(t, err)
	require.Equal(t, testKubeletDigest, digests["kubelet"])
}
