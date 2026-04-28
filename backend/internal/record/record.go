// Package record provides the namespace-based update recording mechanism
// that all game systems use to track changes and construct data packets sent
// to the frontend.
//
// Each game system implements a RecordBucket (for incremental diff collection)
// and registers a SystemProvider (covering both diff and full-snapshot paths).
// During game logic execution, systems write to their bucket within the current
// namespace. After execution, the Registry merges buckets across namespaces and
// produces the data packet.
package record

import "encoding/json"

// RecordBucket is the per-system container that collects changes during a
// single namespace. Each system implements its own bucket with an internal
// data structure optimized for deduplication (e.g. a map keyed by identity).
//
// Buckets at the same namespace level are independent. When namespaces are
// popped, buckets of the same system type are merged via MergeInPlace.
type RecordBucket interface {
	// SystemName identifies the owning system.
	SystemName() string

	// MergeInPlace merges another bucket of the same system type into this one.
	// The other bucket's contents are folded in; after the call, this bucket
	// represents the combined state. The other bucket is not mutated.
	MergeInPlace(other RecordBucket)

	// SerializeDiff serializes this bucket's contents for the incremental
	// diff packet. An empty bucket should return an empty JSON array ("[]").
	SerializeDiff() (json.RawMessage, error)

	// IsEmpty reports whether this bucket contains no records.
	IsEmpty() bool
}

// SystemProvider is the complete contract a game system registers at startup.
// One registration covers both incremental diff and full-snapshot
// serialization, ensuring the wire format stays consistent.
type SystemProvider interface {
	// SystemName returns the stable identifier for this system
	// ("inventory", "attribute", "skill_xp", "event", "bestiary").
	SystemName() string

	// NewBucket returns a new, empty RecordBucket for this system.
	NewBucket() RecordBucket

	// SerializeFull serializes the complete current state of this system for
	// a full-snapshot packet (e.g. on initial connection or reconnect).
	// The state parameter is the opaque state object owned by the system.
	SerializeFull(state any) (json.RawMessage, error)
}
