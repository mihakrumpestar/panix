// Package main implements a benchmark comparison tool that discovers, runs,
// and renders Go benchmark results with multi-variant comparison.
//
// # Naming Convention
//
//	Benchmark_<Variant>__<Feature>    — variant of a benchmark
//	Benchmark__<Feature>              — unnamed/baseline variant
//
// The __ separator splits variant from feature name. All benchmarks sharing
// the same feature name are grouped for comparison, regardless of variant count.
//
// # Sorting
//
// Variant columns are sorted by overall performance. For each feature, every
// variant's time is divided by the fastest time for that feature, producing a
// ratio (1.0 = fastest). The sum of ratios across all features is the
// totalRatio (lower is better). Columns are ordered by ascending totalRatio,
// so the best overall variant appears first.
//
// The header shows the overall multiplier relative to the best variant:
//
//	VariantName +0.15×    — 15% worse cumulative performance than best
//
// # Output Format
//
// One table per package. Each cell shows time, bytes/allocs in parens, and
// ratio vs the leftmost (best overall) variant as base:
//
//	668 ns (0 B, 0)           — base variant, no ratio shown
//	685 ns (0 B, 0) +1.03×    — 1.03× slower than base
//	550 ns (0 B, 0) −1.21×    — 1.21× faster than base (reciprocal)
//
// "-" means no benchmark exists for that variant/feature pair.
//
// # Usage
//
//	go run ./tests/bench/ [flags] [packages...]
//	  -benchtime duration   per benchmark (default 1s)
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var benchLineRe = regexp.MustCompile(`^Benchmark(\S+?)-\d+\s+\d+\s+([\d.]+)\s+ns/op(?:\s+([\d.]+)\s+B/op\s+(\d+)\s+allocs/op)?`)
var metaLineRe = regexp.MustCompile(`^(goos|goarch|pkg|cpu|benchtime|timestamp):\s+(.+?)\s*$`)

type bench struct {
	name, pkg string
	ns        float64
	bytes     float64
	allocs    int
}

type env struct{ goos, goarch, cpu, benchtime, timestamp string }

// featureGroup holds all variants of a single benchmark feature.
type featureGroup struct {
	feature  string
	variants map[string]bench // variant name → benchmark result
	order    []string         // variant order for deterministic display
}

// parseName extracts (feature, variant) from a benchmark name.
// "Foo__Bar" → (feature="Bar", variant="Foo")
// "__Bar"    → (feature="Bar", variant="")
// "NoDunder" → (feature="NoDunder", variant="")
//
// Sub-benchmark paths are stripped: "BufTypes/ByteSliceSlice__WriteLine"
// becomes variant="ByteSliceSlice", feature="WriteLine".
func parseName(name string) (string, string) {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	before, after, ok := strings.Cut(name, "__")
	if !ok {
		return name, ""
	}

	if before == "" {
		return after, ""
	}

	return after, before
}

// displayVariant returns the human-readable name for a variant.
func displayVariant(v string) string {
	if v == "" {
		return "Ours"
	}

	return strings.TrimPrefix(v, "_")
}

func abbrevPkg(pkg string) string {
	return "./" + strings.TrimPrefix(pkg, "github.com/mihakrumpestar/panix/")
}

func parse(data []byte) ([]bench, env, bool) {
	var (
		benchmarks []bench
		envInfo    env
		curPkg     string
	)

	has := false

	setMeta := map[string]func(string){
		"goos":      func(v string) { envInfo.goos = v },
		"goarch":    func(v string) { envInfo.goarch = v },
		"cpu":       func(v string) { envInfo.cpu = v },
		"pkg":       func(v string) { curPkg = v },
		"benchtime": func(v string) { envInfo.benchtime = v },
		"timestamp": func(v string) { envInfo.timestamp = v },
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if match := metaLineRe.FindStringSubmatch(line); match != nil {
			if fn, ok := setMeta[match[1]]; ok {
				fn(match[2])
			}

			continue
		}

		if match := benchLineRe.FindStringSubmatch(line); match != nil {
			has = true
			ns, _ := strconv.ParseFloat(match[2], 64)
			b, _ := strconv.ParseFloat(match[3], 64)
			a, _ := strconv.Atoi(match[4])
			benchmarks = append(benchmarks, bench{match[1], curPkg, ns, b, a})
		}
	}

	if envInfo.timestamp == "" {
		envInfo.timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return benchmarks, envInfo, has
}

// groupVariants groups benchmarks by (pkg, feature) and collects all variants.
func groupVariants(benchmarks []bench) (map[string]*featureGroup, []string) {
	type key struct{ pkg, feature string }

	groups := map[key]*featureGroup{}
	pkgSeen := map[string]bool{}

	var pkgOrder []string

	// Collect all variants per (pkg, feature)
	for _, bch := range benchmarks {
		feature, variant := parseName(bch.name)
		groupKey := key{bch.pkg, feature}

		if _, ok := groups[groupKey]; !ok {
			groups[groupKey] = &featureGroup{
				feature:  feature,
				variants: map[string]bench{},
			}
		}

		group := groups[groupKey]
		if _, exists := group.variants[variant]; !exists {
			group.order = append(group.order, variant)
		}

		group.variants[variant] = bch

		if !pkgSeen[bch.pkg] {
			pkgSeen[bch.pkg] = true
			pkgOrder = append(pkgOrder, bch.pkg)
		}
	}

	sort.Strings(pkgOrder)

	// Convert to pkg-keyed map for display
	result := map[string]*featureGroup{}

	for groupKey, group := range groups {
		pkgKey := groupKey.pkg + "\x00" + groupKey.feature
		result[pkgKey] = group
	}

	return result, pkgOrder
}

//nolint:mnd
func fmtNs(nanos float64) string {
	switch {
	case nanos < 1:
		return fmt.Sprintf("%.2f ns", nanos)
	case nanos < 999.5:
		return fmt.Sprintf("%.0f ns", nanos)
	case nanos < 1e6:
		return fmt.Sprintf("%.1f µs", nanos/1e3)
	case nanos < 1e9:
		return fmt.Sprintf("%.1f ms", nanos/1e6)
	default:
		return fmt.Sprintf("%.2f s", nanos/1e9)
	}
}

//nolint:mnd
func fmtBytes(bytes float64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%.0f B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", bytes/1024)
	default:
		return fmt.Sprintf("%.1f MB", bytes/(1024*1024))
	}
}

//nolint:mnd
func fmtVsBase(baseNs, ns float64) string {
	if baseNs == 0 || ns == 0 {
		return ""
	}

	ratio := ns / baseNs
	switch {
	case ratio < 0.001:
		return fmt.Sprintf("−%.0f×", 1/ratio)
	case ratio < 0.995:
		return fmt.Sprintf("−%.2f×", 1/ratio)
	case ratio < 1.005:
		return ""
	case ratio < 10:
		return fmt.Sprintf("+%.2f×", ratio)
	case ratio < 100:
		return fmt.Sprintf("+%.1f×", ratio)
	default:
		return fmt.Sprintf("+%.0f×", ratio)
	}
}

// ─── Comparison constants ───────────────────────────────────────────────────

const maxDurationNs = 1e18

const (
	nearEqualRatio  = 1.005
	slowRatio       = 10
	muchSlowerRatio = 100
)

//nolint:cyclop,gocognit,gocyclo,funlen,maintidx // table formatting is inherently branchy
func printMultiVariantTable(writer io.Writer, pkg string, groups map[string]*featureGroup) {
	var features []*featureGroup

	for _, group := range groups {
		for _, bch := range group.variants {
			if bch.pkg == pkg {
				features = append(features, group)

				break
			}
		}
	}

	if len(features) == 0 {
		return
	}

	sort.Slice(features, func(i, j int) bool {
		return features[i].feature < features[j].feature
	})

	variantSet := map[string]bool{}

	for _, group := range features {
		for _, variant := range group.order {
			variantSet[variant] = true
		}
	}

	allVariants := make([]string, 0, len(variantSet))
	for variant := range variantSet {
		allVariants = append(allVariants, variant)
	}

	// Score each variant: totalRatio = sum of (variant_ns / fastest_ns)
	// across features (lower is better).
	type variantScore struct {
		name       string
		totalRatio float64
	}

	scores := make(map[string]*variantScore, len(allVariants))
	for _, variant := range allVariants {
		scores[variant] = &variantScore{name: variant}
	}

	for _, group := range features {
		fastestNs := maxDurationNs

		for _, variant := range allVariants {
			if bch, ok := group.variants[variant]; ok && bch.ns < fastestNs {
				fastestNs = bch.ns
			}
		}

		if fastestNs == 0 {
			fastestNs = 1
		}

		for _, variant := range allVariants {
			bch, ok := group.variants[variant]
			if !ok {
				continue
			}

			scores[variant].totalRatio += bch.ns / fastestNs
		}
	}

	sort.Slice(allVariants, func(i, j int) bool {
		return scores[allVariants[i]].totalRatio < scores[allVariants[j]].totalRatio
	})

	// Compute overall multiplier for each variant relative to best.
	bestRatio := scores[allVariants[0]].totalRatio
	if bestRatio == 0 {
		bestRatio = 1
	}

	overallMul := make(map[string]string, len(allVariants))
	for _, variant := range allVariants {
		ratio := scores[variant].totalRatio / bestRatio
		switch {
		case ratio < nearEqualRatio:
			overallMul[variant] = ""
		case ratio < slowRatio:
			overallMul[variant] = fmt.Sprintf(" +%.2f×", ratio-1)
		case ratio < muchSlowerRatio:
			overallMul[variant] = fmt.Sprintf(" +%.1f×", ratio-1)
		default:
			overallMul[variant] = fmt.Sprintf(" +%.0f×", ratio-1)
		}
	}

	cols := 1 + len(allVariants)
	right := make(map[int]bool, cols)

	right[0] = false
	for i := 1; i < cols; i++ {
		right[i] = true
	}

	type row struct{ cells []string }

	var rows []row

	for _, group := range features {
		fastestNs := maxDurationNs

		for _, variant := range allVariants {
			if bch, ok := group.variants[variant]; ok && bch.ns < fastestNs {
				fastestNs = bch.ns
			}
		}

		baseNs := float64(0)
		if bch, ok := group.variants[allVariants[0]]; ok {
			baseNs = bch.ns
		}

		cells := make([]string, cols)
		cells[0] = group.feature

		for idx, variant := range allVariants {
			bch, ok := group.variants[variant]
			if !ok {
				cells[1+idx] = "-"

				continue
			}

			cell := fmt.Sprintf("%s (%s, %d)", fmtNs(bch.ns), fmtBytes(bch.bytes), bch.allocs)

			vs := fmtVsBase(baseNs, bch.ns)
			if vs != "" {
				cell += " " + vs
			}

			cells[1+idx] = cell
		}

		rows = append(rows, row{cells})
	}

	// ─── Print ──────────────────────────────────────────────────────
	hdr := make([]string, cols)

	hdr[0] = "Feature"
	for idx, variant := range allVariants {
		hdr[1+idx] = displayVariant(variant) + overallMul[variant]
	}

	widths := make([]int, len(hdr))
	for i, h := range hdr {
		widths[i] = len(h)
	}

	for _, r := range rows {
		for i, c := range r.cells {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	pad := func(s string, n int, r bool) string {
		if r {
			return fmt.Sprintf("%*s", n, s)
		}

		return fmt.Sprintf("%-*s", n, s)
	}

	printRow := func(row []string) {
		parts := make([]string, len(row))
		for i, c := range row {
			parts[i] = pad(c, widths[i], right[i])
		}

		_, _ = fmt.Fprintln(writer, "  "+strings.Join(parts, " │ "))
	}

	printSep := func() {
		parts := make([]string, len(widths))
		for i, n := range widths {
			parts[i] = strings.Repeat("─", n)
		}

		_, _ = fmt.Fprintln(writer, "  "+strings.Join(parts, "─┼─"))
	}

	_, _ = fmt.Fprintf(writer, "\n  %s\n\n", abbrevPkg(pkg))

	printRow(hdr)
	printSep()

	for _, r := range rows {
		printRow(r.cells)
	}
}

func printFooter(writer io.Writer, envInfo env) {
	sysParts := []string{"goos: " + envInfo.goos, "goarch: " + envInfo.goarch}
	if envInfo.cpu != "" {
		sysParts = append(sysParts, "cpu: "+envInfo.cpu)
	}

	_, _ = fmt.Fprintf(writer, "\n  %s\n", strings.Join(sysParts, "  "))

	if envInfo.benchtime != "" {
		_, _ = fmt.Fprintf(writer, "  benchtime: %s\n", envInfo.benchtime)
	}

	if envInfo.timestamp != "" {
		t, parseErr := time.Parse(time.RFC3339, envInfo.timestamp)

		ts := envInfo.timestamp
		if parseErr == nil {
			ts = t.Format("2006-01-02 15:04:05")
		}

		_, _ = fmt.Fprintf(writer, "  ran: %s\n", ts)
	}
}

func runBenchmarks(ctx context.Context, pkgs []string, benchtime string) []byte {
	results := make([][]byte, len(pkgs))

	for idx, pkg := range pkgs {
		cmd := exec.CommandContext(ctx, "go", "test", "-bench=.", "-benchmem", "-count=1", "-run=^$", "-benchtime="+benchtime, pkg) //nolint:gosec

		var buf bytes.Buffer

		cmd.Stdout = &buf
		_ = cmd.Run()

		results[idx] = buf.Bytes()
	}

	var combined bytes.Buffer
	fmt.Fprintf(&combined, "benchtime: %s\ntimestamp: %s\n", benchtime, time.Now().UTC().Format(time.RFC3339))

	for _, data := range results {
		combined.Write(data)
	}

	return combined.Bytes()
}

func resolvePkgs(ctx context.Context, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	args := append([]string{"list"}, patterns...)

	out, err := exec.CommandContext(ctx, "go", args...).Output() //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	var pkgs []string

	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}

	return pkgs, nil
}

func run() error {
	benchtime := flag.String("benchtime", "1s", "benchmark duration per function")
	outFile := flag.String("out", "tests/bench/results.txt", "output file path (empty to skip file)")

	flag.Parse()

	pkgs, err := resolvePkgs(context.Background(), flag.Args())
	if err != nil {
		return err
	}

	data := runBenchmarks(context.Background(), pkgs, *benchtime)

	benchmarks, envInfo, has := parse(data)
	if !has {
		fmt.Println("No benchmarks found")

		return nil
	}

	groups, pkgOrder := groupVariants(benchmarks)

	// Write to both stdout and file
	var writers = []io.Writer{os.Stdout}

	if *outFile != "" {
		//nolint:gosec,mnd // G301: standard directory permissions
		mkdirErr := os.MkdirAll(strings.TrimSuffix(*outFile, "/"+filepath.Base(*outFile)), 0o755)
		if mkdirErr != nil {
			return fmt.Errorf("create output dir: %w", mkdirErr)
		}

		var file *os.File

		file, err = os.Create(*outFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}

		defer func() { _ = file.Close() }()

		writers = append(writers, file)
	}

	writer := io.MultiWriter(writers...)

	for _, pkg := range pkgOrder {
		printMultiVariantTable(writer, pkg, groups)
	}

	printFooter(writer, envInfo)

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
