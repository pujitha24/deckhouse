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


// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(ms|s|m)$`
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
	IndirectChecks int32 `json:"indirectChecks"`
	// +kubebuilder:validation:Minimum=1
	AwarenessMaxMultiplier int32    `json:"awarenessMaxMultiplier"`
	GossipInterval         Duration `json:"gossipInterval"`
	// +kubebuilder:validation:Minimum=1
	RetransmitMult      int32    `json:"retransmitMult"`
	GossipToTheDeadTime Duration `json:"gossipToTheDeadTime"`
}

// FencingSLAProfileFallback covers the isolated-node path: how often the node
// proves it is alive through the Kubernetes API, and how long that proof holds.
type FencingSLAProfileFallback struct {
	// Heartbeat is how often the agent refreshes status.fallback. Must be well below TTL.
	Heartbeat Duration `json:"heartbeat"`
	// TTL is read by fencing-controller: while the heartbeat is newer than TTL, eviction is blocked.
	TTL Duration `json:"ttl"`
	// KubernetesAPITimeout bounds the API reachability probe. Keep it below TTL so the
	// probe cannot outlive the freshness it is meant to establish.
	KubernetesAPITimeout Duration `json:"kubernetesAPITimeout"`
}

// FencingSLAProfileRejoin bounds the exponential backoff of memberlist.Join retries.
type FencingSLAProfileRejoin struct {
	Interval    Duration `json:"interval"`
	MaxInterval Duration `json:"maxInterval"`
}

// FencingSLAProfileEvacuation is read by fencing-controller, not by the agent.
type FencingSLAProfileEvacuation struct {
	// Delay is counted from status.failed.detectedAt.
	Delay Duration `json:"delay"`
}

// FencingSLAProfileWatchdog drives the local deadman timer.
type FencingSLAProfileWatchdog struct {
	// FeedInterval must be a small fraction of Timeout, so a single late tick cannot expire it.
	FeedInterval Duration `json:"feedInterval"`
	// Timeout is the time from the last feed to the kernel panic.
	Timeout Duration `json:"timeout"`
}


type FencingSLAProfileSpec struct {
	ReactionGoal          string                      `json:"reactionGoal"`
	DetectionWindowTarget string                      `json:"detectionWindowTarget"`
	Memberlist            FencingSLAProfileMemberlist `json:"memberlist"`
	Fallback              FencingSLAProfileFallback   `json:"fallback"`
	Rejoin                FencingSLAProfileRejoin     `json:"rejoin"`
	Evacuation            FencingSLAProfileEvacuation `json:"evacuation"`
	Watchdog              FencingSLAProfileWatchdog   `json:"watchdog"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=fsp
// +kubebuilder:printcolumn:name=Goal,type=string,JSONPath=`.spec.reactionGoal`
// +kubebuilder:printcolumn:name=Detection,type=string,JSONPath=`.spec.detectionWindowTarget`
// +kubebuilder:printcolumn:name=Probe,type=string,JSONPath=`.spec.memberlist.probeInterval`
// +kubebuilder:printcolumn:name=FallbackTTL,type=string,JSONPath=`.spec.fallback.ttl`
// +kubebuilder:printcolumn:name=EvacuationDelay,type=string,JSONPath=`.spec.evacuation.delay`
// +kubebuilder:printcolumn:name=Rejoin,type=string,JSONPath=`.spec.rejoin.interval`
// +kubebuilder:printcolumn:name=Watchdog,type=string,JSONPath=`.spec.watchdog.timeout`
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
