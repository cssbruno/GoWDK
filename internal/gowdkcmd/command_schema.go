package gowdkcmd

import (
	"fmt"
	"sort"
	"strings"
)

const completionUsage = "usage: gowdk completion <bash|zsh|fish>"

// FlagSpec is one completion/documentation view of a parser-owned flag. The
// canonical token comes from the CommandSpec usage line so help, docs, and
// completion cannot drift independently.
type FlagSpec struct {
	Name  string
	Group string
}

// FlagGroup names flags shared by multiple commands. Groups are descriptive;
// command membership is still determined from the command's canonical usage.
type FlagGroup struct {
	Name  string
	Flags []string
}

var commandFlagGroups = []FlagGroup{
	{Name: "project", Flags: []string{"--config", "--project-root", "--env-file", "--module", "--ssr"}},
	{Name: "output", Flags: []string{"--json", "--debug", "--timings"}},
	{Name: "security", Flags: []string{"--allow-insecure", "--allow-missing-backend"}},
}

func inspectCommandSpecs() []CommandSpec {
	names := []string{"ir", "tree", "endpoint-graph", "asset-graph", "go-bindings"}
	children := make([]CommandSpec, 0, len(names))
	for _, name := range names {
		usage := fmt.Sprintf("usage: gowdk inspect %s [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]", name)
		children = append(children, CommandSpec{Name: name, Usage: staticCommandUsage(usage), Summary: "inspect validated compiler data"})
	}
	return children
}

func contractListCommandSpecs() []CommandSpec {
	usage := staticCommandUsage("usage: gowdk list commands|queries|events|jobs [--json] [dir]")
	return []CommandSpec{
		{Name: "commands", Usage: usage, Summary: "list commands"},
		{Name: "queries", Usage: usage, Summary: "list queries"},
		{Name: "events", Usage: usage, Summary: "list events"},
		{Name: "jobs", Usage: usage, Summary: "list jobs"},
	}
}

func playgroundCommandSpecs() []CommandSpec {
	usage := staticCommandUsage(playgroundUsage)
	return []CommandSpec{
		{Name: "policy", Usage: usage, Summary: "inspect the sandbox policy"},
		{Name: "export", Usage: usage, Summary: "export a playground project"},
		{Name: "run", Usage: usage, Summary: "run an opted-in sandbox build"},
	}
}

func completionCommandSpecs() []CommandSpec {
	return []CommandSpec{
		{Name: "bash", Usage: staticCommandUsage("usage: gowdk completion bash"), Summary: "generate Bash completion"},
		{Name: "zsh", Usage: staticCommandUsage("usage: gowdk completion zsh"), Summary: "generate Zsh completion"},
		{Name: "fish", Usage: staticCommandUsage("usage: gowdk completion fish"), Summary: "generate Fish completion"},
	}
}

func commandFlags(spec CommandSpec) []FlagSpec {
	seen := map[string]bool{}
	var flags []FlagSpec
	for _, token := range strings.Fields(spec.Usage()) {
		index := strings.Index(token, "--")
		if index < 0 {
			continue
		}
		token = token[index:]
		end := len(token)
		for offset, char := range token {
			if offset > 1 && !(char == '-' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
				end = offset
				break
			}
		}
		name := token[:end]
		if name == "--" || seen[name] {
			continue
		}
		seen[name] = true
		flags = append(flags, FlagSpec{Name: name, Group: flagGroup(name)})
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func flagGroup(name string) string {
	for _, group := range commandFlagGroups {
		for _, candidate := range group.Flags {
			if candidate == name {
				return group.Name
			}
		}
	}
	return "command"
}

type commandRecord struct {
	Path    []string
	Spec    CommandSpec
	Flags   []FlagSpec
	Summary string
}

func commandRecords() []commandRecord {
	var records []commandRecord
	var visit func([]string, CommandSpec)
	visit = func(parent []string, spec CommandSpec) {
		path := append(append([]string(nil), parent...), spec.Name)
		records = append(records, commandRecord{Path: path, Spec: spec, Flags: commandFlags(spec), Summary: spec.Summary})
		for _, child := range spec.Children {
			visit(path, child)
		}
	}
	for _, spec := range topLevelCommands {
		visit(nil, spec)
	}
	return records
}

func completionCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%s", completionUsage)
	}
	var output string
	switch args[0] {
	case "bash":
		output = bashCompletion()
	case "zsh":
		output = zshCompletion()
	case "fish":
		output = fishCompletion()
	case "markdown":
		output = commandDocumentationMarkdown()
	default:
		return fmt.Errorf("unknown completion shell %q; expected bash, zsh, or fish", args[0])
	}
	fmt.Print(output)
	return nil
}

func bashCompletion() string {
	var lines []string
	lines = append(lines, "_gowdk_complete() {", "  local current=${COMP_WORDS[COMP_CWORD]}", "  local words")
	lines = append(lines, "  case \"${COMP_WORDS[1]}\" in")
	for _, spec := range topLevelCommands {
		var words []string
		for _, child := range spec.Children {
			words = append(words, child.Name)
		}
		for _, flag := range commandFlags(spec) {
			words = append(words, flag.Name)
		}
		lines = append(lines, fmt.Sprintf("    %s) words=%q ;;", spec.Name, strings.Join(words, " ")))
	}
	lines = append(lines, "    *) words=\""+strings.Join(topLevelCommandNames(), " ")+"\" ;;", "  esac", "  COMPREPLY=($(compgen -W \"${words}\" -- \"${current}\"))", "}", "complete -F _gowdk_complete gowdk")
	return strings.Join(lines, "\n") + "\n"
}

func zshCompletion() string {
	var entries []string
	for _, spec := range topLevelCommands {
		entries = append(entries, fmt.Sprintf("'%s:%s'", spec.Name, shellDescription(spec)))
	}
	var lines []string
	lines = append(lines, "#compdef gowdk", "_arguments '1:command:(("+strings.Join(entries, " ")+"))'")
	for _, record := range commandRecords() {
		var flags []string
		for _, flag := range record.Flags {
			flags = append(flags, flag.Name)
		}
		lines = append(lines, "# gowdk "+strings.Join(record.Path, " ")+": "+strings.Join(flags, " "))
	}
	return strings.Join(lines, "\n") + "\n"
}

func fishCompletion() string {
	var lines []string
	for _, record := range commandRecords() {
		if len(record.Path) == 1 {
			lines = append(lines, fmt.Sprintf("complete -c gowdk -n '__fish_use_subcommand' -a %s -d %q", record.Path[0], shellDescription(record.Spec)))
			continue
		}
		parent := strings.Join(record.Path[:len(record.Path)-1], " ")
		lines = append(lines, fmt.Sprintf("complete -c gowdk -n '__fish_seen_subcommand_from %s' -a %s -d %q", strings.ReplaceAll(parent, " ", "' '"), record.Path[len(record.Path)-1], shellDescription(record.Spec)))
	}
	for _, spec := range topLevelCommands {
		for _, flag := range commandFlags(spec) {
			lines = append(lines, fmt.Sprintf("complete -c gowdk -n '__fish_seen_subcommand_from %s' -l %s", spec.Name, strings.TrimPrefix(flag.Name, "--")))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func topLevelCommandNames() []string {
	names := make([]string, 0, len(topLevelCommands))
	for _, spec := range topLevelCommands {
		names = append(names, spec.Name)
	}
	return names
}

func shellDescription(spec CommandSpec) string {
	if strings.TrimSpace(spec.Summary) != "" {
		return strings.ReplaceAll(spec.Summary, "'", "")
	}
	description := strings.TrimSpace(spec.ListSuffix)
	if description == "" {
		return spec.Name + " command"
	}
	return strings.ReplaceAll(description, "'", "")
}

func commandDocumentationMarkdown() string {
	var builder strings.Builder
	builder.WriteString("# CLI Command Schema\n\n")
	builder.WriteString("This file is generated from `internal/gowdkcmd.CommandSpec`.\n\n")
	for _, record := range commandRecords() {
		builder.WriteString("## `gowdk " + strings.Join(record.Path, " ") + "`\n\n")
		builder.WriteString("```text\n" + record.Spec.Usage() + "\n```\n\n")
	}
	return strings.TrimRight(builder.String(), "\n") + "\n"
}
