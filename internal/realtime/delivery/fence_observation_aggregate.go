package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	FenceExpectedNodeManifestSchemaV1        = "dipole.realtime.delivery-fence-expected-nodes.v1"
	FenceObservationAggregateReceiptSchemaV1 = "dipole.realtime.delivery-fence-observation-aggregate.v1"
	FenceObservationAggregateEligible        = "eligible"
)

type FenceExpectedNode struct {
	Component         string    `json:"component"`
	ObserverID        string    `json:"observer_id"`
	ExpectedAuthority Authority `json:"expected_authority,omitempty"`
}

type FenceExpectedNodeManifest struct {
	SchemaVersion string              `json:"schema_version"`
	ManifestID    string              `json:"manifest_id"`
	Nodes         []FenceExpectedNode `json:"nodes"`
}

type FenceObservationAggregateReceipt struct {
	SchemaVersion    string             `json:"schema_version"`
	Decision         string             `json:"decision"`
	ManifestID       string             `json:"manifest_id"`
	ManifestSHA256   string             `json:"manifest_sha256"`
	TransitionID     string             `json:"transition_id"`
	RequestSHA256    string             `json:"request_sha256"`
	LeaseSHA256      string             `json:"lease_sha256"`
	Authority        Authority          `json:"authority"`
	Phase            FencePhase         `json:"phase"`
	Epoch            uint64             `json:"epoch"`
	LeaseUntilUnixMS int64              `json:"lease_until_unix_ms"`
	CapturedAtUnixMS int64              `json:"captured_at_unix_ms"`
	Observations     []FenceObservation `json:"observations"`
}

type RedisFenceObservationAggregator struct {
	client redis.Cmdable
	prefix string
	now    func() time.Time
}

func NewRedisFenceObservationAggregator(client redis.Cmdable, prefix string, now func() time.Time) (*RedisFenceObservationAggregator, error) {
	prefix = strings.TrimSpace(prefix)
	if client == nil || prefix == "" || now == nil {
		return nil, fmt.Errorf("delivery authority observation aggregator configuration is invalid")
	}
	return &RedisFenceObservationAggregator{client: client, prefix: prefix, now: now}, nil
}

func (a *RedisFenceObservationAggregator) Aggregate(
	ctx context.Context,
	manifest FenceExpectedNodeManifest,
	transition FenceTransitionReceipt,
) (FenceObservationAggregateReceipt, error) {
	nodes, manifestSHA256, err := validateExpectedNodeManifest(manifest)
	if err != nil {
		return FenceObservationAggregateReceipt{}, err
	}
	if err := validateAggregateTransitionReceipt(transition); err != nil {
		return FenceObservationAggregateReceipt{}, err
	}
	now := a.now().UTC()
	if !time.UnixMilli(transition.LeaseUntilUnixMS).After(now) {
		return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority transition lease is expired")
	}
	if time.UnixMilli(transition.AppliedAtUnixMS).After(now.Add(2 * time.Second)) {
		return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority transition receipt is from the future")
	}
	if transition.Phase == FencePhaseActive {
		for _, node := range nodes {
			if expectedNodeAuthority(node, transition) != transition.Authority {
				return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority active transition requires every expected node to target %s", transition.Authority)
			}
		}
	}
	observations := make([]FenceObservation, 0, len(nodes))
	for _, node := range nodes {
		key := a.prefix + node.Component + ":" + node.ObserverID
		payload, err := a.client.Get(ctx, key).Bytes()
		if err != nil {
			if err == redis.Nil {
				return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority observation %s/%s is expired or missing", node.Component, node.ObserverID)
			}
			return FenceObservationAggregateReceipt{}, fmt.Errorf("read delivery authority observation %s/%s: %w", node.Component, node.ObserverID, err)
		}
		ttl, err := a.client.PTTL(ctx, key).Result()
		if err != nil || ttl <= 0 {
			return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority observation %s/%s is expired or missing", node.Component, node.ObserverID)
		}
		observation, err := decodeFenceObservation(payload)
		if err != nil {
			return FenceObservationAggregateReceipt{}, fmt.Errorf("decode delivery authority observation %s/%s: %w", node.Component, node.ObserverID, err)
		}
		if err := validateAggregateObservation(observation, node, transition, now); err != nil {
			return FenceObservationAggregateReceipt{}, fmt.Errorf("delivery authority observation %s/%s: %w", node.Component, node.ObserverID, err)
		}
		observations = append(observations, observation)
	}
	return FenceObservationAggregateReceipt{
		SchemaVersion: FenceObservationAggregateReceiptSchemaV1,
		Decision:      FenceObservationAggregateEligible,
		ManifestID:    manifest.ManifestID, ManifestSHA256: manifestSHA256,
		TransitionID: transition.TransitionID, RequestSHA256: transition.RequestSHA256,
		LeaseSHA256: transition.NextSHA256, Authority: transition.Authority, Phase: transition.Phase,
		Epoch: transition.Epoch, LeaseUntilUnixMS: transition.LeaseUntilUnixMS,
		CapturedAtUnixMS: now.UnixMilli(), Observations: observations,
	}, nil
}

func validateExpectedNodeManifest(manifest FenceExpectedNodeManifest) ([]FenceExpectedNode, string, error) {
	manifest.ManifestID = strings.TrimSpace(manifest.ManifestID)
	if manifest.SchemaVersion != FenceExpectedNodeManifestSchemaV1 || !fenceTransitionIDPattern.MatchString(manifest.ManifestID) {
		return nil, "", fmt.Errorf("delivery authority expected-node manifest identity is invalid")
	}
	if len(manifest.Nodes) == 0 || len(manifest.Nodes) > 256 {
		return nil, "", fmt.Errorf("delivery authority expected-node manifest size is invalid")
	}
	nodes := append([]FenceExpectedNode(nil), manifest.Nodes...)
	for index := range nodes {
		nodes[index].Component = strings.TrimSpace(nodes[index].Component)
		nodes[index].ObserverID = strings.TrimSpace(nodes[index].ObserverID)
		if !fenceTransitionIDPattern.MatchString(nodes[index].Component) || !fenceTransitionIDPattern.MatchString(nodes[index].ObserverID) {
			return nil, "", fmt.Errorf("delivery authority expected node identity is invalid")
		}
		if nodes[index].ExpectedAuthority != "" {
			if _, err := ParseAuthority(string(nodes[index].ExpectedAuthority)); err != nil {
				return nil, "", fmt.Errorf("delivery authority expected node authority is invalid")
			}
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Component == nodes[j].Component {
			return nodes[i].ObserverID < nodes[j].ObserverID
		}
		return nodes[i].Component < nodes[j].Component
	})
	for index := 1; index < len(nodes); index++ {
		if nodes[index] == nodes[index-1] {
			return nil, "", fmt.Errorf("delivery authority expected-node manifest contains duplicate node")
		}
	}
	canonical := FenceExpectedNodeManifest{SchemaVersion: manifest.SchemaVersion, ManifestID: manifest.ManifestID, Nodes: nodes}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return nil, "", fmt.Errorf("encode delivery authority expected-node manifest: %w", err)
	}
	return nodes, hashBytes(payload), nil
}

func validateAggregateTransitionReceipt(receipt FenceTransitionReceipt) error {
	if receipt.SchemaVersion != FenceTransitionReceiptSchemaV1 ||
		!fenceTransitionIDPattern.MatchString(receipt.TransitionID) ||
		!fenceTransitionIDPattern.MatchString(receipt.OperatorID) ||
		!validSHA256(receipt.RequestSHA256) || !validSHA256(receipt.ReasonSHA256) ||
		!validSHA256(receipt.NextSHA256) ||
		(receipt.PreviousSHA256 != "" && !validSHA256(receipt.PreviousSHA256)) ||
		receipt.Epoch == 0 || receipt.LeaseUntilUnixMS <= 0 || receipt.AppliedAtUnixMS <= 0 {
		return fmt.Errorf("delivery authority transition receipt is invalid")
	}
	if _, err := ParseAuthority(string(receipt.Authority)); err != nil {
		return err
	}
	if receipt.Phase != FencePhaseActive && receipt.Phase != FencePhaseFrozen {
		return fmt.Errorf("delivery authority transition receipt phase is invalid")
	}
	switch receipt.Action {
	case FenceTransitionBootstrap:
		if receipt.Authority != AuthorityGo || receipt.Phase != FencePhaseActive || receipt.Epoch != 1 || receipt.PreviousSHA256 != "" {
			return fmt.Errorf("delivery authority bootstrap receipt state is invalid")
		}
	case FenceTransitionFreeze:
		if receipt.Phase != FencePhaseFrozen || receipt.PreviousSHA256 == "" {
			return fmt.Errorf("delivery authority freeze receipt state is invalid")
		}
	case FenceTransitionActivate:
		if receipt.Phase != FencePhaseActive || receipt.PreviousSHA256 == "" {
			return fmt.Errorf("delivery authority activate receipt state is invalid")
		}
	case FenceTransitionRenew:
		if receipt.PreviousSHA256 == "" {
			return fmt.Errorf("delivery authority renewal receipt state is invalid")
		}
	default:
		return fmt.Errorf("delivery authority transition receipt action is invalid")
	}
	return nil
}

func decodeFenceObservation(payload []byte) (FenceObservation, error) {
	return DecodeStrictJSON[FenceObservation](payload)
}

func validateAggregateObservation(observation FenceObservation, node FenceExpectedNode, transition FenceTransitionReceipt, now time.Time) error {
	if observation.SchemaVersion != FenceObservationSchemaV1 || observation.Component != node.Component || observation.ObserverID != node.ObserverID {
		return fmt.Errorf("identity is invalid")
	}
	if observation.ExpectedAuthority != expectedNodeAuthority(node, transition) || observation.ObservedAuthority != transition.Authority ||
		observation.ExpectedEpoch != transition.Epoch || observation.ObservedEpoch != transition.Epoch {
		return fmt.Errorf("authority or epoch does not match transition")
	}
	if observation.ObservedPhase != transition.Phase || observation.ObservedLeaseUntilUnixMS != transition.LeaseUntilUnixMS ||
		observation.ObservedLeaseSHA256 != transition.NextSHA256 {
		return fmt.Errorf("lease does not match transition")
	}
	switch transition.Phase {
	case FencePhaseActive:
		if observation.Status != FenceObservationAuthorized || observation.ReasonCode != FenceReasonAuthorized {
			return fmt.Errorf("status does not authorize active lease")
		}
	case FencePhaseFrozen:
		if observation.Status != FenceObservationDenied || observation.ReasonCode != FenceReasonFrozen {
			return fmt.Errorf("status does not confirm frozen lease")
		}
	}
	observedAt := time.UnixMilli(observation.ObservedAtUnixMS)
	expiresAt := time.UnixMilli(observation.ExpiresAtUnixMS)
	observationTTL := expiresAt.Sub(observedAt)
	if observation.ObservedAtUnixMS <= 0 || observation.ExpiresAtUnixMS <= 0 ||
		observationTTL < 5*time.Second || observationTTL > time.Minute ||
		observedAt.After(now.Add(2*time.Second)) || !expiresAt.After(now) {
		return fmt.Errorf("observation is expired or has invalid lifetime")
	}
	return nil
}

func expectedNodeAuthority(node FenceExpectedNode, transition FenceTransitionReceipt) Authority {
	if node.ExpectedAuthority != "" {
		return node.ExpectedAuthority
	}
	return transition.Authority
}
