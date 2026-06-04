package actions

import (
	"sort"

	yaml "gopkg.in/yaml.v3"
)

// workflowTriggers holds the trigger events and cron schedules declared in a
// workflow's `on:` section.
type workflowTriggers struct {
	Events []string
	Crons  []string
}

// parseWorkflowTriggers extracts trigger events and schedule cron expressions
// from raw workflow YAML. It is resilient to the YAML 1.1 quirk where the bare
// key `on` is resolved as the boolean true.
func parseWorkflowTriggers(raw string) workflowTriggers {
	var result workflowTriggers
	if raw == "" {
		return result
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return result
	}
	if len(doc.Content) == 0 {
		return result
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return result
	}

	var onNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		// Match "on" whether parsed as a string or as the boolean true.
		if key.Value == "on" || (key.Tag == "!!bool" && key.Value == "true") {
			onNode = root.Content[i+1]
			break
		}
	}
	if onNode == nil {
		return result
	}

	eventSet := map[string]bool{}
	switch onNode.Kind {
	case yaml.ScalarNode:
		eventSet[onNode.Value] = true
	case yaml.SequenceNode:
		for _, item := range onNode.Content {
			eventSet[item.Value] = true
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(onNode.Content); i += 2 {
			event := onNode.Content[i].Value
			eventSet[event] = true
			if event == "schedule" {
				result.Crons = append(result.Crons, extractCrons(onNode.Content[i+1])...)
			}
		}
	}

	for e := range eventSet {
		if e != "" {
			result.Events = append(result.Events, e)
		}
	}
	sort.Strings(result.Events)
	return result
}

// extractCrons pulls cron strings from a schedule node, which is a sequence of
// mappings like `- cron: '0 0 * * *'`.
func extractCrons(scheduleNode *yaml.Node) []string {
	var crons []string
	if scheduleNode.Kind != yaml.SequenceNode {
		return crons
	}
	for _, item := range scheduleNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value == "cron" {
				if v := item.Content[i+1].Value; v != "" {
					crons = append(crons, v)
				}
			}
		}
	}
	return crons
}
