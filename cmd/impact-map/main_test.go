package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

func TestVersionCommandPrintsStampedVersion(t *testing.T) {
	oldVersion := version
	version = "v1.2.3"
	t.Cleanup(func() { version = oldVersion })

	root := newRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := out.String(), "impact-map v1.2.3\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestIsNewerVersionComparesSemanticTags(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{current: "v0.1.0", latest: "v0.2.0", want: true},
		{current: "0.2.0", latest: "v0.2.0", want: false},
		{current: "v0.10.0", latest: "v0.9.0", want: false},
		{current: "dev", latest: "v0.2.0", want: true},
		{current: "v1.0.0", latest: "v1.0.0", want: false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.current, tt.latest), func(t *testing.T) {
			if got := isNewerVersion(tt.current, tt.latest); got != tt.want {
				t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestUpdateCacheRoundTripAndStaleness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest_version")
	checkedAt := time.Now().Add(-23 * time.Hour).Truncate(time.Second)

	if err := writeLatestVersionCache(path, latestVersionCache{CheckedAt: checkedAt, Version: "v0.2.0"}); err != nil {
		t.Fatalf("writeLatestVersionCache() error = %v", err)
	}

	got, ok, err := readFreshLatestVersionCache(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("readFreshLatestVersionCache() error = %v", err)
	}
	if !ok {
		t.Fatal("readFreshLatestVersionCache() ok = false, want true")
	}
	if got.Version != "v0.2.0" || !got.CheckedAt.Equal(checkedAt) {
		t.Fatalf("cache = %#v, want version v0.2.0 checkedAt %v", got, checkedAt)
	}

	if err := writeLatestVersionCache(path, latestVersionCache{CheckedAt: time.Now().Add(-25 * time.Hour), Version: "v0.3.0"}); err != nil {
		t.Fatalf("write stale cache error = %v", err)
	}
	_, ok, err = readFreshLatestVersionCache(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("read stale cache error = %v", err)
	}
	if ok {
		t.Fatal("stale cache ok = true, want false")
	}
}

func TestBackgroundUpdateNoticeUsesFreshCache(t *testing.T) {
	oldVersion := version
	version = "v0.1.0"
	t.Cleanup(func() { version = oldVersion })

	path := filepath.Join(t.TempDir(), "latest_version")
	if err := writeLatestVersionCache(path, latestVersionCache{CheckedAt: time.Now(), Version: "v0.2.0"}); err != nil {
		t.Fatalf("write cache error = %v", err)
	}

	var stderr bytes.Buffer
	check := startBackgroundUpdateCheck(context.Background(), updateCheckConfig{
		CachePath: path,
		Stderr:    &stderr,
	})
	check.PrintNotice()

	want := "impact-map v0.1.0: update available v0.2.0. Run `impact-map upgrade`\n"
	if got := stderr.String(); got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
}

func TestVerifyArchiveChecksumMatchesChecksumsFile(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "impact-map_darwin_arm64.tar.gz")
	if err := os.WriteFile(archive, []byte("archive"), 0o644); err != nil {
		t.Fatalf("write archive error = %v", err)
	}

	checksums := "0eb3e36bfb24dcd9bb1d1bece1531216b59539a8fde17ee80224af0653c92aa3  impact-map_darwin_arm64.tar.gz\n"
	if err := verifyArchiveChecksum(archive, []byte(checksums)); err != nil {
		t.Fatalf("verifyArchiveChecksum() error = %v", err)
	}
}

func TestReportGraphAndCyclesChangedFilesOnlyFiltersCyclesToKeptNodes(t *testing.T) {
	g := analyzer.NewGraph()
	nodeA := &analyzer.Node{ID: "a", Path: "components/A.vue"}
	nodeB := &analyzer.Node{ID: "b", Path: "components/B.vue"}
	nodeC := &analyzer.Node{ID: "c", Path: "components/C.vue"}
	nodeD := &analyzer.Node{ID: "d", Path: "components/D.vue"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)
	g.AddNode(nodeD)

	g.AddEdge(analyzer.Edge{From: "a", To: "b"})
	g.AddEdge(analyzer.Edge{From: "b", To: "a"})
	g.AddEdge(analyzer.Edge{From: "c", To: "d"})
	g.AddEdge(analyzer.Edge{From: "d", To: "c"})

	reportGraph, circular := reportGraphAndCycles(g, true, []string{"a"}, []string{"b"})

	if len(reportGraph.Nodes) != 2 {
		t.Fatalf("filtered graph node count = %d, want 2", len(reportGraph.Nodes))
	}
	if _, ok := reportGraph.Nodes["c"]; ok {
		t.Fatal("filtered graph contains unexpected node c")
	}
	if len(circular) != 1 {
		t.Fatalf("circular len = %d, want 1; got %#v", len(circular), circular)
	}
	for _, id := range circular[0] {
		if _, ok := reportGraph.Nodes[id]; !ok {
			t.Fatalf("cycle references node %q missing from filtered graph", id)
		}
	}
}

func TestReportGraphAndCyclesFullGraphKeepsOriginalCycles(t *testing.T) {
	g := analyzer.NewGraph()
	nodeA := &analyzer.Node{ID: "a", Path: "components/A.vue"}
	nodeB := &analyzer.Node{ID: "b", Path: "components/B.vue"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddEdge(analyzer.Edge{From: "a", To: "b"})
	g.AddEdge(analyzer.Edge{From: "b", To: "a"})

	reportGraph, circular := reportGraphAndCycles(g, false, []string{"a"}, []string{"b"})

	if reportGraph != g {
		t.Fatal("full report graph should reuse original graph")
	}
	if len(circular) != 1 {
		t.Fatalf("circular len = %d, want 1", len(circular))
	}
}
