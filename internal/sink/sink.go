// Package sink is the optional, opt-in outbound path: it ships audit
// decisions to a self-hosted collector (hard rule 3).
//
// The sink is off unless <data-dir>/sink.json exists. The audit log stays
// the source of truth: the shipper reads audit.jsonl from a byte offset,
// POSTs new decisions in batches and moves the offset only after the
// collector answered 2xx. A failed or interrupted send is simply retried
// later (at-least-once); every event carries an ID (the SHA-256 of its
// audit line) so the collector can drop duplicates.
package sink

import "github.com/pilsu-choi/mcpgraph/internal/model"

// Event is one decision on the wire: the decision's own fields plus an ID.
// Embedding model.Decision makes encoding/json write its fields at the
// same level as "id" (embedded struct fields are flattened).
type Event struct {
	ID string `json:"id"`
	model.Decision
}

// Batch is the body of POST /v1/events.
type Batch struct {
	Host   string  `json:"host"`
	Events []Event `json:"events"`
}
