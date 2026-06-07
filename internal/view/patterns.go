package view

import "regexp"

var (
	islandFieldPattern         = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	islandIncDecPattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$`)
	islandAssignPattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	islandTogglePattern        = regexp.MustCompile(`^!\s*([A-Za-z_][A-Za-z0-9_]*)$`)
	islandNumberPattern        = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
	islandTextBindingPattern   = regexp.MustCompile(`^\s*\{([A-Za-z_][A-Za-z0-9_]*)\}\s*$`)
	islandRefCallPattern       = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$`)
	islandLetPattern           = regexp.MustCompile(`^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	islandAwaitFetchPattern    = regexp.MustCompile(`^await\s+fetchJSON\[(.+)\]\((.*)\)$`)
	forDirectivePattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(?:\s*,\s*([A-Za-z_][A-Za-z0-9_]*))?\s+in\s+(.+)$`)
	contractReferencePattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+$`)
	eventNamePattern           = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	stylePropertyPattern       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	styleCustomPropertyPattern = regexp.MustCompile(`^--[A-Za-z0-9_-]+$`)
)
