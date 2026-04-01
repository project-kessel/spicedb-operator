package main

import (
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/authzed/spicedb-operator/pkg/config"
	"github.com/authzed/spicedb-operator/pkg/updates"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args, err := parseArgs()
	if err != nil {
		return err
	}

	opConfig, err := loadGraph(args.graphFile)
	if err != nil {
		return fmt.Errorf("loading update graph from %s: %w", args.graphFile, err)
	}

	graph := &opConfig.UpdateGraph

	// If listing datastores/versions, handle that and exit
	if args.listDatastores {
		return listDatastores(graph)
	}
	if args.listVersions {
		return listVersions(graph, args.datastore)
	}

	// Validate upgrade path
	if args.from == "" || args.to == "" {
		return fmt.Errorf("both --from and --to are required (use --help for usage)")
	}

	return validateUpgrade(graph, args.datastore, args.from, args.to)
}

type cliArgs struct {
	graphFile      string
	datastore      string
	from           string
	to             string
	listDatastores bool
	listVersions   bool
}

func parseArgs() (cliArgs, error) {
	args := cliArgs{
		graphFile: "config/update-graph.yaml",
		datastore: "postgres",
	}

	positional := make([]string, 0)
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--graph", "-g":
			i++
			if i >= len(os.Args) {
				return args, fmt.Errorf("--graph requires a value")
			}
			args.graphFile = os.Args[i]
		case "--datastore", "-d":
			i++
			if i >= len(os.Args) {
				return args, fmt.Errorf("--datastore requires a value")
			}
			args.datastore = os.Args[i]
		case "--list-datastores":
			args.listDatastores = true
		case "--list-versions":
			args.listVersions = true
		default:
			positional = append(positional, os.Args[i])
		}
	}

	if !args.listDatastores && !args.listVersions {
		if len(positional) == 2 {
			args.from = positional[0]
			args.to = positional[1]
		} else if len(positional) != 0 {
			return args, fmt.Errorf("expected exactly 2 positional args: FROM_VERSION TO_VERSION (got %d)", len(positional))
		}
	}

	return args, nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `validate-upgrade-path - Validate SpiceDB upgrade paths against the update graph

USAGE:
  validate-upgrade-path [FLAGS] FROM_VERSION TO_VERSION

  Checks whether upgrading from FROM_VERSION to TO_VERSION is supported by the
  update graph for a given datastore. Shows the full step-by-step upgrade path
  including any required migrations.

  This is useful when managing your own SpiceDB images outside of the operator's
  built-in update mechanism, to ensure you don't skip required migration steps.

FLAGS:
  -g, --graph FILE        Path to update-graph.yaml (default: config/update-graph.yaml)
  -d, --datastore NAME    Datastore type: postgres, cockroachdb, mysql, spanner, memory
                          (default: postgres)
      --list-datastores   List available datastores and exit
      --list-versions     List all versions for the given datastore and exit
  -h, --help              Show this help

EXAMPLES:
  # Check if upgrading from v1.29.5 to v1.35.3 is valid for postgres
  validate-upgrade-path v1.29.5 v1.35.3

  # Check upgrade path for cockroachdb
  validate-upgrade-path -d cockroachdb v1.29.5 v1.35.3

  # Use a different graph file (e.g. from an upstream bundle)
  validate-upgrade-path -g /path/to/update-graph.yaml v1.29.5 v1.35.3

  # List available datastores
  validate-upgrade-path --list-datastores

  # List all versions for a datastore
  validate-upgrade-path --list-versions -d postgres
`)
}

func loadGraph(path string) (*config.OperatorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var opConfig config.OperatorConfig
	if err := yaml.Unmarshal(data, &opConfig); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	if len(opConfig.Channels) == 0 {
		return nil, fmt.Errorf("no channels found in %s", path)
	}

	return &opConfig, nil
}

func listDatastores(graph *updates.UpdateGraph) error {
	seen := make(map[string]bool)
	for _, ch := range graph.Channels {
		ds := ch.Metadata[updates.DatastoreMetadataKey]
		if ds != "" && !seen[ds] {
			fmt.Printf("  %s (channel: %s)\n", ds, ch.Name)
			seen[ds] = true
		}
	}
	return nil
}

func listVersions(graph *updates.UpdateGraph, datastore string) error {
	for _, ch := range graph.Channels {
		if !strings.EqualFold(ch.Metadata[updates.DatastoreMetadataKey], datastore) {
			continue
		}
		fmt.Printf("Versions for %s (channel: %s):\n", datastore, ch.Name)
		for _, node := range ch.Nodes {
			migrationInfo := ""
			if node.Migration != "" {
				migrationInfo = fmt.Sprintf("  migration: %s", node.Migration)
			}
			phaseInfo := ""
			if node.Phase != "" {
				phaseInfo = fmt.Sprintf("  phase: %s", node.Phase)
			}
			fmt.Printf("  %s%s%s\n", node.ID, migrationInfo, phaseInfo)
		}
		return nil
	}
	return fmt.Errorf("no channel found for datastore %q", datastore)
}

func validateUpgrade(graph *updates.UpdateGraph, datastore, from, to string) error {
	channelName, err := graph.DefaultChannelForDatastore(datastore)
	if err != nil {
		return fmt.Errorf("datastore %q not found in update graph", datastore)
	}

	source, err := graph.SourceForChannel(datastore, channelName)
	if err != nil {
		return err
	}

	// Check that both versions exist in the graph
	fromState := source.State(from)
	if fromState.ID == "" {
		return fmt.Errorf("version %q not found in the %s channel for %s", from, channelName, datastore)
	}
	toState := source.State(to)
	if toState.ID == "" {
		return fmt.Errorf("version %q not found in the %s channel for %s", to, channelName, datastore)
	}

	// Try to find a path by walking edges from `from` toward `to`
	// First create a subgraph with `to` as the head, then walk from `from`
	subSource, err := source.Subgraph(to)
	if err != nil {
		fmt.Printf("UPGRADE NOT SUPPORTED: no path from %s to %s for %s\n", from, to, datastore)
		fmt.Println("\nThe update graph does not define a valid upgrade path between these versions.")
		return nil
	}

	// Walk the path from `from` to `to`
	path := []pathStep{{
		version:   from,
		migration: fromState.Migration,
		phase:     fromState.Phase,
	}}

	current := from
	for current != to {
		next := subSource.NextVersion(current)
		if next == "" {
			fmt.Printf("UPGRADE NOT SUPPORTED: no path from %s to %s for %s\n", from, to, datastore)
			fmt.Println("\nThe update graph does not define a valid upgrade path between these versions.")
			fmt.Println("You may need to upgrade through intermediate versions first.")
			return nil
		}
		nextState := source.State(next)
		step := pathStep{
			version:   next,
			migration: nextState.Migration,
			phase:     nextState.Phase,
		}

		// Determine if this step requires a migration
		prevState := source.State(current)
		if nextState.Migration != prevState.Migration || nextState.Phase != prevState.Phase {
			step.hasMigration = true
		}

		path = append(path, step)
		current = next
	}

	// Print results
	if len(path) <= 1 {
		fmt.Printf("SAME VERSION: %s is already at %s\n", from, to)
		return nil
	}

	migrationCount := 0
	for _, s := range path[1:] {
		if s.hasMigration {
			migrationCount++
		}
	}

	fmt.Printf("UPGRADE SUPPORTED: %s -> %s for %s\n", from, to, datastore)
	fmt.Printf("  Steps: %d\n", len(path)-1)
	fmt.Printf("  Migrations required: %d\n", migrationCount)
	fmt.Println()

	// Print the path
	fmt.Println("Upgrade path:")
	for i, step := range path {
		prefix := "  "
		if i == 0 {
			prefix = "  [current] "
		} else if i == len(path)-1 {
			prefix = "  [target]  "
		} else {
			prefix = "  [step]    "
		}

		migNote := ""
		if i > 0 && step.hasMigration {
			migNote = fmt.Sprintf(" (migration: %s", step.migration)
			if step.phase != "" {
				migNote += fmt.Sprintf(", phase: %s", step.phase)
			}
			migNote += ")"
		} else if i > 0 {
			migNote = " (no migration)"
		}

		fmt.Printf("%s%s%s\n", prefix, step.version, migNote)
	}

	if migrationCount > 0 {
		fmt.Println()
		fmt.Println("NOTE: This upgrade requires database migrations. Ensure you follow each step")
		fmt.Println("      in sequence and do not skip intermediate versions.")
	}

	return nil
}

type pathStep struct {
	version      string
	migration    string
	phase        string
	hasMigration bool
}
