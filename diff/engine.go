package diff

// AXNode represents a node from ax-tree.json.
type AXNode struct {
	NodeID   string         `json:"nodeId"`
	Role     string         `json:"role"`
	Name     string         `json:"name"`
	Children []string       `json:"children,omitempty"`
	Props    map[string]any `json:"properties,omitempty"`
}

// ElementChange describes a single element-level change.
type ElementChange struct {
	Key    string `json:"key"`    // "role:name" identity
	Role   string `json:"role"`
	Name   string `json:"name"`
	Change string `json:"change"` // "appeared", "disappeared"
}

// AuditDelta describes per-category audit count changes.
type AuditDelta struct {
	Category string `json:"category"`
	Before   int    `json:"before"`
	After    int    `json:"after"`
	Delta    int    `json:"delta"`
}

// URLDiff is the diff result for a single URL.
type URLDiff struct {
	Path           string          `json:"path"`
	AuditDeltas    []AuditDelta    `json:"auditDeltas"`
	ElementChanges []ElementChange `json:"elementChanges"`
	HasRegressions bool            `json:"hasRegressions"`
}

// Result is the top-level diff output.
type Result struct {
	BaselineID  string    `json:"baselineId"`
	CurrentID   string    `json:"currentId"`
	URLs        []URLDiff `json:"urls"`
	Regressions bool      `json:"regressions"`
}

// DiffAudit compares two audit JSON results and returns deltas.
func DiffAudit(baseline, current map[string]any) []AuditDelta {
	categories := []string{"contrastViolations", "touchViolations", "typographyWarnings"}
	var deltas []AuditDelta

	baseSummary, _ := getMap(baseline, "summary")
	currSummary, _ := getMap(current, "summary")

	for _, cat := range categories {
		before := getInt(baseSummary, cat)
		after := getInt(currSummary, cat)
		deltas = append(deltas, AuditDelta{
			Category: cat,
			Before:   before,
			After:    after,
			Delta:    after - before,
		})
	}
	return deltas
}

// DiffAxTree compares two ax-tree JSON results and returns element changes.
func DiffAxTree(baseline, current map[string]any) []ElementChange {
	baseNodes := extractNodes(baseline)
	currNodes := extractNodes(current)

	var changes []ElementChange

	for key, node := range currNodes {
		if _, exists := baseNodes[key]; !exists {
			changes = append(changes, ElementChange{
				Key:    key,
				Role:   node.Role,
				Name:   node.Name,
				Change: "appeared",
			})
		}
	}
	for key, node := range baseNodes {
		if _, exists := currNodes[key]; !exists {
			changes = append(changes, ElementChange{
				Key:    key,
				Role:   node.Role,
				Name:   node.Name,
				Change: "disappeared",
			})
		}
	}
	return changes
}

// HasRegressions returns true if any audit delta is positive or any element disappeared.
func HasRegressions(auditDeltas []AuditDelta, elemChanges []ElementChange) bool {
	for _, d := range auditDeltas {
		if d.Delta > 0 {
			return true
		}
	}
	for _, e := range elemChanges {
		if e.Change == "disappeared" {
			return true
		}
	}
	return false
}

func extractNodes(axTree map[string]any) map[string]AXNode {
	result := make(map[string]AXNode)
	nodesList, ok := axTree["nodes"]
	if !ok {
		return result
	}
	nodes, ok := nodesList.([]any)
	if !ok {
		return result
	}
	for _, n := range nodes {
		nm, ok := n.(map[string]any)
		if !ok {
			continue
		}
		role, _ := nm["role"].(string)
		name, _ := nm["name"].(string)
		nodeID, _ := nm["nodeId"].(string)
		if role == "" {
			continue
		}
		key := role + ":" + name
		result[key] = AXNode{NodeID: nodeID, Role: role, Name: name}
	}
	return result
}

func getMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	r, ok := v.(map[string]any)
	return r, ok
}

func getInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
