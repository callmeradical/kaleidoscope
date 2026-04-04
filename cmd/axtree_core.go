package cmd

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// AXNode is a simplified accessibility tree node for snapshot storage.
type AXNode struct {
	NodeID     string            `json:"nodeId"`
	Role       string            `json:"role"`
	Name       string            `json:"name"`
	Children   []string          `json:"children,omitempty"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// runAxTreeOnPage retrieves and converts the full accessibility tree for page.
func runAxTreeOnPage(page *rod.Page) ([]AXNode, error) {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, err
	}

	nodes := make([]AXNode, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		if node.Ignored {
			continue
		}
		n := AXNode{
			NodeID: string(node.NodeID),
		}
		if node.Role != nil {
			if s, ok := node.Role.Value.Val().(string); ok {
				n.Role = s
			}
		}
		if node.Name != nil {
			if s, ok := node.Name.Value.Val().(string); ok {
				n.Name = s
			}
		}
		if len(node.ChildIDs) > 0 {
			children := make([]string, len(node.ChildIDs))
			for i, id := range node.ChildIDs {
				children[i] = string(id)
			}
			n.Children = children
		}
		if len(node.Properties) > 0 {
			props := make(map[string]any)
			for _, p := range node.Properties {
				props[string(p.Name)] = p.Value.Value
			}
			n.Properties = props
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}
