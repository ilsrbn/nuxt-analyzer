package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
	"github.com/ilsrbn/nuxt-analyzer/internal/cache"
	"github.com/ilsrbn/nuxt-analyzer/internal/git"
	"github.com/ilsrbn/nuxt-analyzer/internal/impact"
	"github.com/ilsrbn/nuxt-analyzer/internal/parser"
	"github.com/ilsrbn/nuxt-analyzer/internal/reporter"
)

var version = "dev"

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "impact-map",
		Short: "Nuxt release impact analyzer",
	}
	root.PersistentFlags().Bool("no-color", false, "disable colored output")

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze blast radius of git changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			updateCheck := startBackgroundUpdateCheck(cmd.Context(), updateCheckConfig{
				Stderr: cmd.ErrOrStderr(),
			})
			err := runAnalyze(cmd, args)
			updateCheck.PrintNotice()
			return err
		},
		SilenceUsage: true,
	}

	analyze.Flags().String("base", "origin/main", "git base ref")
	analyze.Flags().String("head", "HEAD", "git head ref")
	analyze.Flags().String("project-root", ".", "path to Nuxt project root")
	analyze.Flags().String("output", "", "write JSON to file (default: stdout)")
	analyze.Flags().Bool("no-cache", false, "skip cache lookup")
	analyze.Flags().Bool("include-tests", false, "include test files")
	analyze.Flags().Bool("json-pretty", false, "pretty-print JSON output")
	analyze.Flags().Bool("changed-files-only", false, "output only changed nodes")

	root.AddCommand(analyze)
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "impact-map %s\n", version)
			return err
		},
	})
	root.AddCommand(&cobra.Command{
		Use:          "upgrade",
		Short:        "Upgrade impact-map to the latest release",
		RunE:         runUpgrade,
		SilenceUsage: true,
	})

	return root
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	start := time.Now()

	base, err := cmd.Flags().GetString("base")
	if err != nil {
		return fmt.Errorf("read --base: %w", err)
	}

	head, err := cmd.Flags().GetString("head")
	if err != nil {
		return fmt.Errorf("read --head: %w", err)
	}

	projectRoot, err := cmd.Flags().GetString("project-root")
	if err != nil {
		return fmt.Errorf("read --project-root: %w", err)
	}

	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("read --output: %w", err)
	}

	noCache, err := cmd.Flags().GetBool("no-cache")
	if err != nil {
		return fmt.Errorf("read --no-cache: %w", err)
	}

	includeTests, err := cmd.Flags().GetBool("include-tests")
	if err != nil {
		return fmt.Errorf("read --include-tests: %w", err)
	}

	pretty, err := cmd.Flags().GetBool("json-pretty")
	if err != nil {
		return fmt.Errorf("read --json-pretty: %w", err)
	}

	changedFilesOnly, err := cmd.Flags().GetBool("changed-files-only")
	if err != nil {
		return fmt.Errorf("read --changed-files-only: %w", err)
	}

	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	useCache := !noCache
	cacheKey := cache.CacheKey(base, head)
	reportCache := cache.NoopCache{}
	if useCache {
		if cached, ok := reportCache.Get(cacheKey); ok {
			if !json.Valid(cached) {
				return fmt.Errorf("cached report: invalid JSON")
			}
			return writeOutput(output, cached)
		}
	}

	changedFiles, err := git.New(projectRoot).ChangedFiles(base, head)
	if err != nil {
		return fmt.Errorf("git diff: %w", err)
	}

	files, err := analyzer.Scan(projectRoot, includeTests)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	bridge, err := parser.New()
	if err != nil {
		return fmt.Errorf("parser bridge init: %w", err)
	}
	defer bridge.Cleanup()

	builder := analyzer.Builder{ProjectRoot: projectRoot}
	graph, buildErrs, err := builder.Build(files, bridge)
	if err != nil {
		return fmt.Errorf("build graph: %w", err)
	}

	reportErrors := make([]reporter.ReportError, 0, len(buildErrs))
	for _, buildErr := range buildErrs {
		reportErrors = append(reportErrors, reporter.ReportError{
			File:    buildErr.File,
			Message: buildErr.Message,
			Kind:    buildErr.Kind,
		})
	}

	changedIDs := mapChangedFiles(changedFiles, graph)
	for _, id := range changedIDs {
		if node := graph.Nodes[id]; node != nil {
			node.IsChanged = true
		}
	}

	engine := &impact.Engine{}
	impactResult := engine.Analyze(graph, changedIDs)
	severityResult := impact.Score(graph, impactResult)

	reportGraph, circular := reportGraphAndCycles(graph, changedFilesOnly, impactResult.ChangedNodes, impactResult.AffectedNodes)

	report := reporter.Build(reporter.BuildInput{
		ProjectName:    filepath.Base(projectRoot),
		ProjectRoot:    projectRoot,
		GitBase:        base,
		GitHead:        head,
		ChangedFiles:   changedFiles,
		Graph:          reportGraph,
		ImpactResult:   impactResult,
		SeverityResult: severityResult,
		Circular:       circular,
		Errors:         reportErrors,
		StartTime:      start,
	})

	data, err := reporter.Marshal(report, pretty)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if useCache {
		if err := reportCache.Set(cacheKey, data); err != nil {
			return fmt.Errorf("cache report: %w", err)
		}
	}

	return writeOutput(output, data)
}

func writeOutput(output string, data []byte) error {
	if output == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			return fmt.Errorf("write stdout newline: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(output, data, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func mapChangedFiles(changedFiles []string, g *analyzer.Graph) []string {
	if g == nil {
		return []string{}
	}

	ids := make([]string, 0, len(changedFiles))
	seen := make(map[string]struct{}, len(changedFiles))
	for _, changedFile := range changedFiles {
		id := analyzer.NodeID(filepath.ToSlash(changedFile))
		if _, ok := g.Nodes[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids
}

func filteredGraph(g *analyzer.Graph, changedIDs, affectedIDs []string) *analyzer.Graph {
	filtered := analyzer.NewGraph()
	if g == nil {
		return filtered
	}

	keep := make(map[string]struct{}, len(changedIDs)+len(affectedIDs))
	for _, id := range changedIDs {
		keep[id] = struct{}{}
	}
	for _, id := range affectedIDs {
		keep[id] = struct{}{}
	}

	for id, node := range g.Nodes {
		if _, ok := keep[id]; !ok {
			continue
		}
		filtered.AddNode(node)
	}

	for _, edge := range g.Edges {
		if _, ok := keep[edge.From]; !ok {
			continue
		}
		if _, ok := keep[edge.To]; !ok {
			continue
		}
		filtered.AddEdge(edge)
	}

	return filtered
}

func reportGraphAndCycles(g *analyzer.Graph, changedFilesOnly bool, changedIDs, affectedIDs []string) (*analyzer.Graph, [][]string) {
	if !changedFilesOnly {
		return g, impact.FindCircular(g)
	}

	filtered := filteredGraph(g, changedIDs, affectedIDs)
	return filtered, impact.FindCircular(filtered)
}
