package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunAxTree(args []string) {
	err := browser.WithPage(func(page *rod.Page) error {
		// Use CDP Accessibility domain to get the full tree
		tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
		if err != nil {
			return err
		}

		// Convert to a simpler format for output
		nodes := make([]map[string]any, 0)
		for _, node := range tree.Nodes {
			n := map[string]any{
				"nodeId": node.NodeID,
				"role":   "",
				"name":   "",
			}
			if node.Role != nil {
				n["role"] = node.Role.Value
			}
			if node.Name != nil {
				n["name"] = node.Name.Value
			}
			if node.Ignored {
				continue
			}
			if len(node.ChildIDs) > 0 {
				children := make([]string, len(node.ChildIDs))
				for i, id := range node.ChildIDs {
					children[i] = string(id)
				}
				n["children"] = children
			}
			if len(node.Properties) > 0 {
				props := make(map[string]any)
				for _, p := range node.Properties {
					props[string(p.Name)] = p.Value.Value
				}
				n["properties"] = props
			}
			nodes = append(nodes, n)
		}

		output.Success("ax-tree", map[string]any{
			"nodeCount": len(nodes),
			"nodes":     nodes,
		})
		return nil
	})

	if err != nil {
		output.Fail("ax-tree", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
