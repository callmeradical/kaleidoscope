package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

// gatherAxTreeData dumps the full ax-tree from the current page and returns the result map.
// Does NOT call output.Success or os.Exit.
func gatherAxTreeData(page *rod.Page) (map[string]any, error) {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, err
	}

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

	return map[string]any{
		"nodeCount": len(nodes),
		"nodes":     nodes,
	}, nil
}

func RunAxTree(args []string) {
	err := browser.WithPage(func(page *rod.Page) error {
		result, err := gatherAxTreeData(page)
		if err != nil {
			return err
		}
		output.Success("ax-tree", result)
		return nil
	})

	if err != nil {
		output.Fail("ax-tree", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
