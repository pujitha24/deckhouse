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

package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:validation:Pattern=`^[1-9][0-9]*(ms|s|m|h)$`
type Duration string

func (d Duration) Parse() (time.Duration, error) {
	parsed, err := time.ParseDuration(string(d))
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", string(d), err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", string(d))
	}

	return parsed, nil
}

func (p ProfileName) ResourceName() string {
	return strings.ToLower(string(p))
}

type FencingSLAProfileMemberlist struct {
	ProbeInterval Duration `json:"probeInterval"`
	// ProbeTimeout must stay below ProbeInterval and above the p99 RTT of the network.
	ProbeTimeout Duration `json:"probeTimeout"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMult int32 `json:"suspicionMult"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMaxTimeoutMult int32 `json:"suspicionMaxTimeoutMult"`
	// +kubebuilder:validation:Minimum=1
	AwarenessMaxMultiplier int32 `json:"awarenessMaxMultiplier"`
	// +kubebuilder:validation:Minimum=1
	IndirectChecks int32    `json:"indirectChecks"`
	GossipInterval Duration `json:"gossipInterval"`
	// +kubebuilder:validation:Minimum=1
	RetransmitMult      int32    `json:"retransmitMult"`
	GossipToTheDeadTime Duration `json:"gossipToTheDeadTime"`
}

type FencingSLAProfileFallback struct {
	// Heartbeat is how often the agent refreshes status.fallback. Must be well below TTL.
	Heartbeat Duration `json:"heartbeat"`
	// APITimeout bounds the Kubernetes API reachability probe. Keep it below TTL so the
	// probe cannot outlive the freshness it is meant to establish.
	APITimeout Duration `json:"apiTimeout"`
	// TTL is read by fencing-controller: while the heartbeat is newer than TTL, eviction is blocked.
	TTL Duration `json:"ttl"`
}

type FencingSLAProfileRejoin struct {
	Interval    Duration `json:"interval"`
	MaxInterval Duration `json:"maxInterval"`
}

type FencingSLAProfileWatchdog struct {
	FeedInterval Duration `json:"feedInterval"`
	// +kubebuilder:validation:Pattern=`^[1-9][0-9]*(s|m|h)$`
	Timeout Duration `json:"timeout"`
}

type FencingSLAProfileEvacuation struct {
	Delay Duration `json:"delay"`
}

type FencingSLAProfileSpec struct {
	Memberlist FencingSLAProfileMemberlist `json:"memberlist"`
	Fallback   FencingSLAProfileFallback   `json:"fallback"`
	Rejoin     FencingSLAProfileRejoin     `json:"rejoin"`
	Watchdog   FencingSLAProfileWatchdog   `json:"watchdog"`
	Evacuation FencingSLAProfileEvacuation `json:"evacuation"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name=ProbeInterval,type=string,JSONPath=`.spec.memberlist.probeInterval`
// +kubebuilder:printcolumn:name=WatchdogTimeout,type=string,JSONPath=`.spec.watchdog.timeout`
// +kubebuilder:printcolumn:name=EvacuationDelay,type=string,JSONPath=`.spec.evacuation.delay`
type FencingSLAProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FencingSLAProfileSpec `json:"spec"`
}

// +kubebuilder:object:root=true
type FencingSLAProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingSLAProfile `json:"items"`
}

var (
	_ runtime.Object = (*FencingSLAProfile)(nil)
	_ runtime.Object = (*FencingSLAProfileList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingSLAProfile{}, &FencingSLAProfileList{})
}
