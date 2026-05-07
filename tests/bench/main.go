// Package main implements a benchmark comparison tool that discovers, runs,
// and renders Go benchmark results comparing custom vs reference implementations.
//
// # Naming Convention
//
//	Ours:  Benchmark__<Feature>        ↔ BenchmarkRef_<Lib>__<Feature>
//
// The __ separator splits prefix from feature name. Ref_ prefix marks reference
// benchmarks. Library name between Ref_ and __ is auto-detected.
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
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var benchLineRe = regexp.MustCompile(`^Benchmark(\S+?)-\d+\s+\d+\s+([\d.]+)\s+ns/op(?:\s+\d+\s+B/op\s+(\d+)\s+allocs/op)?`)
var metaLineRe = regexp.MustCompile(`^(goos|goarch|pkg|cpu|benchtime|timestamp):\s+(.+?)\s*$`)

type bench struct {
	name, pkg string
	ns        float64
	allocs    int
}
type pair struct {
	name      string
	ours, ref bench
	lib       string
}
type env struct{ goos, goarch, cpu, benchtime, timestamp string }

func parseName(name string) (string, string, bool) {
	before, after, ok := strings.Cut(name, "__")
	if !ok {
		return name, "", false
	}

	prefix, feat := before, after

	after, ok = strings.CutPrefix(prefix, "Ref_")
	if ok {
		return feat, after, true
	}

	return feat, "", false
}

func abbrevPkg(pkg string) string {
	return "./" + strings.TrimPrefix(pkg, "github.com/mihakrumpestar/panix/")
}

//nolint:cyclop
func parse(data []byte) ([]bench, env, bool) {
	var (
		benchmarks []bench
		envInfo    env
		curPkg     string
	)

	has := false

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if match := metaLineRe.FindStringSubmatch(line); match != nil {
			switch match[1] {
			case "goos":
				envInfo.goos = match[2]
			case "goarch":
				envInfo.goarch = match[2]
			case "cpu":
				envInfo.cpu = match[2]
			case "pkg":
				curPkg = match[2]
			case "benchtime":
				envInfo.benchtime = match[2]
			case "timestamp":
				envInfo.timestamp = match[2]
			}

			continue
		}

		if match := benchLineRe.FindStringSubmatch(line); match != nil {
			has = true
			ns, _ := strconv.ParseFloat(match[2], 64)
			a, _ := strconv.Atoi(match[3])
			benchmarks = append(benchmarks, bench{match[1], curPkg, ns, a})
		}
	}

	if envInfo.timestamp == "" {
		envInfo.timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return benchmarks, envInfo, has
}

func pairUp(benchmarks []bench) ([]pair, []bench) {
	type key struct{ f, p string }

	oursIdx := map[key]bench{}
	oursUsed := map[key]bool{}

	var (
		pairs    []pair
		unpaired []bench
	)

	for _, bch := range benchmarks {
		feature, _, ref := parseName(bch.name)
		if ref {
			continue
		}

		k := key{feature, bch.pkg}
		if _, ok := oursIdx[k]; !ok {
			oursIdx[k] = bch
		}
	}

	for _, bch := range benchmarks {
		feature, lib, ref := parseName(bch.name)
		if !ref {
			continue
		}

		k := key{feature, bch.pkg}
		if o, ok := oursIdx[k]; ok {
			oursUsed[k] = true

			pairs = append(pairs, pair{feature, o, bch, lib})
		}
	}

	for _, bch := range benchmarks {
		if _, _, ref := parseName(bch.name); ref {
			continue
		}

		feature, _, _ := parseName(bch.name)
		if !oursUsed[key{feature, bch.pkg}] {
			unpaired = append(unpaired, bch)
		}
	}

	srt := func(s func(int) string) func(int, int) bool {
		return func(i, j int) bool { return s(i) < s(j) }
	}
	sort.Slice(pairs, srt(func(i int) string { return pairs[i].ours.pkg + pairs[i].name }))
	sort.Slice(unpaired, srt(func(i int) string { return unpaired[i].pkg + unpaired[i].name }))

	return pairs, unpaired
}

//nolint:mnd
func fmtNs(nanos float64) string {
	switch {
	case nanos < 1:
		return fmt.Sprintf("%.2f ns", nanos)
	case nanos < 1e3:
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
func fmtRatio(o, r float64) string {
	if o == 0 {
		return "-"
	}

	ratio := r / o
	switch {
	case ratio >= 100:
		return fmt.Sprintf("%.0f×", ratio)
	case ratio >= 10:
		return fmt.Sprintf("%.1f×", ratio)
	case ratio >= 1:
		return fmt.Sprintf("%.2f×", ratio)
	default:
		return fmt.Sprintf("%.3f×", ratio)
	}
}

func printTable(pkg string, pairs []pair, unpaired []bench) {
	hdr := []string{"Benchmark", "Ours", "Ref", "Speedup", "Allocs", "RefAllocs", "A-Ratio"}
	right := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true}

	rows := buildTableRows(pairs, unpaired)

	if len(rows) == 0 {
		return
	}

	widths := computeColumnWidths(hdr, rows)
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

		fmt.Println(strings.Join(parts, " │ "))
	}
	printSep := func() {
		parts := make([]string, len(widths))
		for i, n := range widths {
			parts[i] = strings.Repeat("─", n)
		}

		fmt.Println(strings.Join(parts, "─┼─"))
	}

	fmt.Printf("\n  %s\n\n", abbrevPkg(pkg))
	printRow(hdr)
	printSep()

	for _, r := range rows {
		printRow(r)
	}
}

func buildTableRows(pairs []pair, unpaired []bench) [][]string {
	var rows [][]string
	for _, pr := range pairs {
		rows = append(rows, []string{
			pr.name, fmtNs(pr.ours.ns), fmtNs(pr.ref.ns), fmtRatio(pr.ours.ns, pr.ref.ns),
			strconv.Itoa(pr.ours.allocs), strconv.Itoa(pr.ref.allocs), fmtRatio(float64(pr.ours.allocs), float64(pr.ref.allocs)),
		})
	}

	for _, bch := range unpaired {
		feature, _, _ := parseName(bch.name)
		rows = append(rows, []string{feature, fmtNs(bch.ns), "-", "-", strconv.Itoa(bch.allocs), "-", "-"})
	}

	return rows
}

func computeColumnWidths(hdr []string, rows [][]string) []int {
	widths := make([]int, len(hdr))
	for i, h := range hdr {
		widths[i] = len(h)
	}

	for _, r := range rows {
		for i, c := range r {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	return widths
}

func printFooter(envInfo env) {
	sysParts := []string{"goos: " + envInfo.goos, "goarch: " + envInfo.goarch}
	if envInfo.cpu != "" {
		sysParts = append(sysParts, "cpu: "+envInfo.cpu)
	}

	fmt.Printf("\n  %s\n", strings.Join(sysParts, "  "))

	if envInfo.benchtime != "" {
		fmt.Printf("  benchtime: %s\n", envInfo.benchtime)
	}

	if envInfo.timestamp != "" {
		t, err := time.Parse(time.RFC3339, envInfo.timestamp)

		ts := envInfo.timestamp
		if err == nil {
			ts = t.Format("2006-01-02 15:04:05")
		}

		fmt.Printf("  ran: %s\n", ts)
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

	pairs, unpaired := pairUp(benchmarks)
	pkgOrder, groups := groupByPackage(pairs, unpaired)

	for _, pkg := range pkgOrder {
		grp := groups[pkg]
		printTable(pkg, grp.pairs, grp.unpaired)
	}

	printFooter(envInfo)

	return nil
}

type benchGroup struct {
	pairs    []pair
	unpaired []bench
}

func groupByPackage(pairs []pair, unpaired []bench) ([]string, map[string]*benchGroup) {
	groups := map[string]*benchGroup{}

	var pkgOrder []string

	for _, pairItem := range pairs {
		pkg := pairItem.ours.pkg
		if _, ok := groups[pkg]; !ok {
			groups[pkg] = &benchGroup{}
			pkgOrder = append(pkgOrder, pkg)
		}

		groups[pkg].pairs = append(groups[pkg].pairs, pairItem)
	}

	for _, bch := range unpaired {
		if _, ok := groups[bch.pkg]; !ok {
			groups[bch.pkg] = &benchGroup{}
			pkgOrder = append(pkgOrder, bch.pkg)
		}

		groups[bch.pkg].unpaired = append(groups[bch.pkg].unpaired, bch)
	}

	sort.Strings(pkgOrder)

	return pkgOrder, groups
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
