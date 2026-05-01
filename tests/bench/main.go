// Package main implements a benchmark comparison tool that parses Go test -bench
// output and renders a grouped, aligned table showing custom implementations
// vs reference library implementations.
//
// # Benchmark Naming Convention
//
// Paired benchmarks (ours vs reference) are auto-detected by prefix:
//
//	Benchmark<Class>...         → "ours" (custom implementation)
//	BenchmarkLipgloss<Class>... → "reference" (lipgloss/charm.land library)
//	BenchmarkBubbles<Class>...  → "reference" (bubbles library)
//
// The tool strips the "Lipgloss" or "Bubbles" prefix, then matches the
// remaining name with either the exact stripped name or "Simple" + stripped name.
//
// # Pairing Examples
//
//	BenchmarkView                    ↔ BenchmarkBubblesView
//	BenchmarkSimpleTree              ↔ BenchmarkLipglossTree
//	BenchmarkSimpleTreeComparison_2x2x4 ↔ BenchmarkLipglossTreeComparison_2x2x4
//	BenchmarkSetContentLines         ↔ BenchmarkBubblesSetContentLines
//
// # Adding New Comparison Benchmarks
//
// When adding a new comparison benchmark, always add both variants:
//
//	func BenchmarkSimpleWidget(b *testing.B)   { ... } // ours
//	func BenchmarkLipglossWidget(b *testing.B) { ... } // reference
//
// or:
//
//	func BenchmarkWidget(b *testing.B)      { ... } // ours
//	func BenchmarkBubblesWidget(b *testing.B) { ... } // reference
//
// Unpaired benchmarks (no reference counterpart) display with "-" in the
// Ref columns. They are still useful for tracking performance regressions.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	nsPerUs    = 1e3
	nsPerMs    = 1e6
	nsPerSec   = 1e9
	ratioHuge  = 100
	ratioLarge = 10
	ratioUnit  = 1
)

var (
	benchLineRe  = regexp.MustCompile(`^Benchmark(\S+?)-\d+\s+\d+\s+([\d.]+)\s+ns/op(?:\s+\d+\s+B/op\s+(\d+)\s+allocs/op)?`)
	pkgLineRe    = regexp.MustCompile(`^pkg:\s+(.+)`)
	goosLineRe   = regexp.MustCompile(`^goos:\s+(.+)`)
	goarchLineRe = regexp.MustCompile(`^goarch:\s+(.+)`)
	cpuLineRe    = regexp.MustCompile(`^cpu:\s+(.+)`)
)

func abbreviatePackage(pkgPath string) string {
	return strings.TrimPrefix(pkgPath, "github.com/mihakrumpestar/panix/")
}

type benchmark struct {
	name    string
	pkg     string
	nsPerOp float64
	allocs  int
}

type environment struct {
	goos   string
	goarch string
	pkg    string
	cpu    string
}

type pairResult struct {
	name string
	ours benchmark
	ref  benchmark
}

type packageGroup struct {
	pkg      string
	pairs    []pairResult
	unpaired []benchmark
}

func parseBenchmarkOutput(reader *bufio.Reader) ([]benchmark, environment, bool) {
	var results []benchmark

	env := environment{}
	hasData := false

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		if match := goosLineRe.FindStringSubmatch(line); match != nil {
			env.goos = match[1]

			continue
		}

		if match := goarchLineRe.FindStringSubmatch(line); match != nil {
			env.goarch = match[1]

			continue
		}

		if match := cpuLineRe.FindStringSubmatch(line); match != nil {
			env.cpu = match[1]

			continue
		}

		if match := pkgLineRe.FindStringSubmatch(line); match != nil {
			env.pkg = match[1]

			continue
		}

		if match := benchLineRe.FindStringSubmatch(line); match != nil {
			hasData = true
			nsPerOp, _ := strconv.ParseFloat(match[2], 64)

			allocs := 0
			if match[3] != "" {
				allocs, _ = strconv.Atoi(match[3])
			}

			results = append(results, benchmark{
				name:    match[1],
				pkg:     env.pkg,
				nsPerOp: nsPerOp,
				allocs:  allocs,
			})

			continue
		}
	}

	return results, env, hasData
}

func isReference(name string) bool {
	return strings.HasPrefix(name, "Bubbles") || strings.HasPrefix(name, "Lipgloss")
}

func classify(results []benchmark) (map[string][]benchmark, map[string][]benchmark) {
	ours := make(map[string][]benchmark)
	refs := make(map[string][]benchmark)

	for _, result := range results {
		if isReference(result.name) {
			refs[result.name] = append(refs[result.name], result)
		} else {
			ours[result.name] = append(ours[result.name], result)
		}
	}

	return ours, refs
}

func stripReferencePrefix(name string) string {
	for _, prefix := range []string{"Bubbles", "Lipgloss"} {
		if strings.HasPrefix(name, prefix) {
			return name[len(prefix):]
		}
	}

	return name
}

func buildIndex(ours map[string][]benchmark) map[string]benchmark {
	index := make(map[string]benchmark)

	for name, results := range ours {
		for _, result := range results {
			key := name + "\n" + result.pkg
			if _, exists := index[key]; !exists {
				index[key] = result
			}
		}
	}

	return index
}

func tryPair(ref benchmark, candidateNames []string, index map[string]benchmark, matched map[string]bool) (pairResult, bool) {
	for _, candidate := range candidateNames {
		key := candidate + "\n" + ref.pkg
		if currentOurs, found := index[key]; found {
			matched[currentOurs.name] = true

			return pairResult{
				name: candidate,
				ours: currentOurs,
				ref:  ref,
			}, true
		}
	}

	return pairResult{}, false
}

func findRefCandidates(refName string) (string, []string) {
	refBase := stripReferencePrefix(refName)

	return refBase, []string{refBase, "Simple" + refBase}
}

func pairBenchmarks(ours, refs map[string][]benchmark) ([]pairResult, []benchmark) {
	var pairs []pairResult

	matched := make(map[string]bool)

	index := buildIndex(ours)

	for refName, refResults := range refs {
		displayName, candidateNames := findRefCandidates(refName)

		for _, ref := range refResults {
			pair, ok := tryPair(ref, candidateNames, index, matched)
			if ok {
				pair.name = displayName
				pairs = append(pairs, pair)
			}
		}
	}

	// Collect unpaired
	var unpaired []benchmark

	for _, results := range ours {
		for _, result := range results {
			if !matched[result.name] {
				unpaired = append(unpaired, result)
			}
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].ours.pkg != pairs[j].ours.pkg {
			return pairs[i].ours.pkg < pairs[j].ours.pkg
		}

		return pairs[i].name < pairs[j].name
	})

	sort.Slice(unpaired, func(i, j int) bool {
		if unpaired[i].pkg != unpaired[j].pkg {
			return unpaired[i].pkg < unpaired[j].pkg
		}

		return unpaired[i].name < unpaired[j].name
	})

	return pairs, unpaired
}

func groupByPackage(pairs []pairResult, unpaired []benchmark) (map[string]*packageGroup, []string) {
	groups := make(map[string]*packageGroup)
	pkgKeys := make([]string, 0)

	for _, pair := range pairs {
		pkg := pair.ours.pkg
		if _, exists := groups[pkg]; !exists {
			groups[pkg] = &packageGroup{pkg: pkg}
			pkgKeys = append(pkgKeys, pkg)
		}

		groups[pkg].pairs = append(groups[pkg].pairs, pair)
	}

	for _, bench := range unpaired {
		if _, exists := groups[bench.pkg]; !exists {
			groups[bench.pkg] = &packageGroup{pkg: bench.pkg}
			pkgKeys = append(pkgKeys, bench.pkg)
		}

		groups[bench.pkg].unpaired = append(groups[bench.pkg].unpaired, bench)
	}

	sort.Strings(pkgKeys)

	return groups, pkgKeys
}

func formatNanos(nanos float64) string {
	switch {
	case nanos < 1:
		return fmt.Sprintf("%.2f ns", nanos)
	case nanos < nsPerUs:
		return fmt.Sprintf("%.0f ns", nanos)
	case nanos < nsPerMs:
		return fmt.Sprintf("%.1f µs", nanos/nsPerUs)
	case nanos < nsPerSec:
		return fmt.Sprintf("%.1f ms", nanos/nsPerMs)
	default:
		return fmt.Sprintf("%.2f s", nanos/nsPerSec)
	}
}

func formatRatio(ours, ref float64) string {
	if ours == 0 {
		return "-"
	}

	ratio := ref / ours
	switch {
	case ratio >= ratioHuge:
		return fmt.Sprintf("%.0f×", ratio)
	case ratio >= ratioLarge:
		return fmt.Sprintf("%.1f×", ratio)
	case ratio >= ratioUnit:
		return fmt.Sprintf("%.2f×", ratio)
	default:
		return fmt.Sprintf("%.3f×", ratio)
	}
}

func formatAllocRatio(ours, ref int) string {
	if ours == 0 {
		return "-"
	}

	return formatRatio(float64(ours), float64(ref))
}

func pad(s string, width int, right bool) string {
	if right {
		return fmt.Sprintf("%*s", width, s)
	}

	return fmt.Sprintf("%-*s", width, s)
}

func computeCellWidths(header []string, rowData [][]string) []int {
	widths := make([]int, len(header))
	for index, headerCell := range header {
		widths[index] = len(headerCell)
	}

	for _, row := range rowData {
		for index, cell := range row {
			if len(cell) > widths[index] {
				widths[index] = len(cell)
			}
		}
	}

	return widths
}

func printSeparator(widths []int) {
	parts := make([]string, len(widths))
	for index, width := range widths {
		parts[index] = strings.Repeat("─", width)
	}

	fmt.Println(strings.Join(parts, "─┼─"))
}

func printRow(row []string, widths []int, rightAligned map[int]bool) {
	parts := make([]string, len(row))
	for index, cell := range row {
		parts[index] = pad(cell, widths[index], rightAligned[index])
	}

	fmt.Println(strings.Join(parts, " │ "))
}

func printGroup(group *packageGroup) bool {
	header := []string{"Benchmark", "Ours", "Ref", "Speedup", "Allocs", "RefAllocs", "A-Ratio"}
	rightAligned := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true}

	var rows [][]string
	for _, pair := range group.pairs {
		rows = append(rows, []string{
			pair.name,
			formatNanos(pair.ours.nsPerOp),
			formatNanos(pair.ref.nsPerOp),
			formatRatio(pair.ours.nsPerOp, pair.ref.nsPerOp),
			strconv.Itoa(pair.ours.allocs),
			strconv.Itoa(pair.ref.allocs),
			formatAllocRatio(pair.ours.allocs, pair.ref.allocs),
		})
	}

	for _, bench := range group.unpaired {
		rows = append(rows, []string{
			bench.name,
			formatNanos(bench.nsPerOp),
			"-",
			"-",
			strconv.Itoa(bench.allocs),
			"-",
			"-",
		})
	}

	if len(rows) == 0 {
		return false
	}

	widths := computeCellWidths(header, rows)

	abbrev := abbreviatePackage(group.pkg)
	fmt.Printf("\n  %s\n\n", abbrev)
	printRow(header, widths, rightAligned)
	printSeparator(widths)

	for _, row := range rows {
		printRow(row, widths, rightAligned)
	}

	return true
}

func printSummary(env environment) {
	fmt.Printf("\n  goos: %s  goarch: %s", env.goos, env.goarch)

	if env.cpu != "" {
		fmt.Printf("  cpu: %s", env.cpu)
	}

	fmt.Println()
}

func run() {
	reader := bufio.NewReader(os.Stdin)
	results, env, hasData := parseBenchmarkOutput(reader)

	if !hasData {
		fmt.Println("No benchmarks found")

		return
	}

	ours, refs := classify(results)
	pairs, unpaired := pairBenchmarks(ours, refs)
	groups, keys := groupByPackage(pairs, unpaired)

	for _, key := range keys {
		printGroup(groups[key])
	}

	printSummary(env)
}

func main() {
	run()
}
