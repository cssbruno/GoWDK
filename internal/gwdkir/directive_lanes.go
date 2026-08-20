package gwdkir

import (
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/viewmodel"
)

// CollectDirectiveLanes returns source-order structural directive lanes.
func CollectDirectiveLanes(nodes []viewmodel.Node) []DirectiveLane {
	var lanes []DirectiveLane
	var walk func([]viewmodel.Node, string)
	walk = func(items []viewmodel.Node, parent string) {
		for index, node := range items {
			path := strconv.Itoa(index)
			if parent != "" {
				path = parent + "." + path
			}
			var attrs []viewmodel.Attr
			var children []viewmodel.Node
			switch typed := node.(type) {
			case viewmodel.Element:
				attrs, children = typed.Attrs, typed.Children
			case viewmodel.ComponentCall:
				attrs, children = typed.Attrs, typed.Children
			case viewmodel.AwaitBlock:
				walk(typed.Pending, path+".pending")
				walk(typed.Then, path+".then")
				walk(typed.Catch, path+".catch")
				continue
			default:
				continue
			}
			lane := ""
			for _, attr := range attrs {
				if attr.Name == "g:lane" {
					lane = strings.TrimSpace(attr.Value)
				}
			}
			for _, attr := range attrs {
				if attr.Name == "g:for" || attr.Name == "g:if" {
					lanes = append(lanes, DirectiveLane{Path: path, Directive: attr.Name, Lane: lane, Expression: strings.TrimSpace(attr.Value)})
				}
			}
			walk(children, path)
		}
	}
	walk(nodes, "")
	return lanes
}
