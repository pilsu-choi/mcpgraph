// Package policies holds the runtime policy shipped with mcpgraph.
package policies

import _ "embed" // the blank import enables the //go:embed directive

// Default is policies/default.yaml, compiled into the binary so mcpgraph
// works without a policy file on disk.
//
//go:embed default.yaml
var Default []byte
