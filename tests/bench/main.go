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
//	  -benchtime duration   per benchmark (default 500ms)
//	  -parallel bool        run packages in parallel (default true)
//	  -cores int            max parallel packages, 0=unlimited (default 0)
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
	"sync"
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
	name string
	ours, ref bench
	lib  string
}
type env struct{ goos, goarch, cpu, benchtime, timestamp string }

func parseName(name string) (feature, lib string, isRef bool) {
	i := strings.Index(name, "__")
	if i < 0 {
		return name, "", false
	}
	prefix, feat := name[:i], name[i+2:]
	if strings.HasPrefix(prefix, "Ref_") {
		return feat, strings.TrimPrefix(prefix, "Ref_"), true
	}
	return feat, "", false
}

func abbrevPkg(p string) string { return strings.TrimPrefix(p, "github.com/mihakrumpestar/panix/") }

func parse(data []byte) ([]bench, env, bool) {
	var bs []bench
	var e env
	var curPkg string
	has := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if m := metaLineRe.FindStringSubmatch(line); m != nil {
			switch m[1] {
			case "goos":
				e.goos = m[2]
			case "goarch":
				e.goarch = m[2]
			case "cpu":
				e.cpu = m[2]
			case "pkg":
				curPkg = m[2]
			case "benchtime":
				e.benchtime = m[2]
			case "timestamp":
				e.timestamp = m[2]
			}
			continue
		}
		if m := benchLineRe.FindStringSubmatch(line); m != nil {
			has = true
			ns, _ := strconv.ParseFloat(m[2], 64)
			a, _ := strconv.Atoi(m[3])
			bs = append(bs, bench{m[1], curPkg, ns, a})
		}
	}
	if e.timestamp == "" {
		e.timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	return bs, e, has
}

func pairUp(bs []bench) (pairs []pair, unpaired []bench) {
	type key struct{ f, p string }
	oursIdx := map[key]bench{}
	oursUsed := map[key]bool{}

	for _, b := range bs {
		f, _, ref := parseName(b.name)
		if ref {
			continue
		}
		k := key{f, b.pkg}
		if _, ok := oursIdx[k]; !ok {
			oursIdx[k] = b
		}
	}
	for _, b := range bs {
		f, lib, ref := parseName(b.name)
		if !ref {
			continue
		}
		k := key{f, b.pkg}
		if o, ok := oursIdx[k]; ok {
			oursUsed[k] = true
			pairs = append(pairs, pair{f, o, b, lib})
		}
	}
	for _, b := range bs {
		if _, _, ref := parseName(b.name); ref {
			continue
		}
		f, _, _ := parseName(b.name)
		if !oursUsed[key{f, b.pkg}] {
			unpaired = append(unpaired, b)
		}
	}
	srt := func(s func(int) string) func(int, int) bool {
		return func(i, j int) bool { return s(i) < s(j) }
	}
	sort.Slice(pairs, srt(func(i int) string { return pairs[i].ours.pkg + pairs[i].name }))
	sort.Slice(unpaired, srt(func(i int) string { return unpaired[i].pkg + unpaired[i].name }))
	return
}

func fmtNs(n float64) string {
	switch {
	case n < 1:
		return fmt.Sprintf("%.2f ns", n)
	case n < 1e3:
		return fmt.Sprintf("%.0f ns", n)
	case n < 1e6:
		return fmt.Sprintf("%.1f µs", n/1e3)
	case n < 1e9:
		return fmt.Sprintf("%.1f ms", n/1e6)
	default:
		return fmt.Sprintf("%.2f s", n/1e9)
	}
}

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

	var rows [][]string
	for _, p := range pairs {
		rows = append(rows, []string{
			p.name, fmtNs(p.ours.ns), fmtNs(p.ref.ns), fmtRatio(p.ours.ns, p.ref.ns),
			strconv.Itoa(p.ours.allocs), strconv.Itoa(p.ref.allocs), fmtRatio(float64(p.ours.allocs), float64(p.ref.allocs)),
		})
	}
	for _, b := range unpaired {
		f, _, _ := parseName(b.name)
		rows = append(rows, []string{f, fmtNs(b.ns), "-", "-", strconv.Itoa(b.allocs), "-", "-"})
	}
	if len(rows) == 0 {
		return
	}

	w := make([]int, len(hdr))
	for i, h := range hdr {
		w[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if len(c) > w[i] {
				w[i] = len(c)
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
			parts[i] = pad(c, w[i], right[i])
		}
		fmt.Println(strings.Join(parts, " │ "))
	}
	printSep := func() {
		parts := make([]string, len(w))
		for i, n := range w {
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

func printFooter(e env) {
	sysParts := []string{"goos: " + e.goos, "goarch: " + e.goarch}
	if e.cpu != "" {
		sysParts = append(sysParts, "cpu: "+e.cpu)
	}
	fmt.Printf("\n  %s\n", strings.Join(sysParts, "  "))
	if e.benchtime != "" {
		fmt.Printf("  benchtime: %s\n", e.benchtime)
	}
	if e.timestamp != "" {
		t, err := time.Parse(time.RFC3339, e.timestamp)
		ts := e.timestamp
		if err == nil {
			ts = t.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("  ran: %s\n", ts)
	}
}

func runBenchmarks(ctx context.Context, pkgs []string, benchtime string, parallel bool, cores int) ([]byte, error) {
	var sem chan struct{}
	if parallel && cores > 0 {
		sem = make(chan struct{}, cores)
	}

	type result struct {
		data []byte
		ok   bool
	}
	results := make([]result, len(pkgs))
	var wg sync.WaitGroup

	for i, pkg := range pkgs {
		wg.Add(1)
		go func(idx int, pkg string) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			cmd := exec.CommandContext(ctx, "go", "test", "-bench=.", "-benchmem", "-count=1", "-run=^$", "-benchtime="+benchtime, pkg)
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Run()
			results[idx] = result{buf.Bytes(), true}
		}(i, pkg)
	}
	wg.Wait()

	var combined bytes.Buffer
	fmt.Fprintf(&combined, "benchtime: %s\ntimestamp: %s\n", benchtime, time.Now().UTC().Format(time.RFC3339))
	for _, r := range results {
		combined.Write(r.data)
	}
	return combined.Bytes(), nil
}

func resolvePkgs(ctx context.Context, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	args := append([]string{"list"}, patterns...)
	out, err := exec.CommandContext(ctx, "go", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}
	var pkgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

func run() error {
	benchtime := flag.String("benchtime", "500ms", "benchmark duration per function")
	parallel := flag.Bool("parallel", true, "run packages in parallel")
	cores := flag.Int("cores", 0, "max parallel packages (0 = unlimited)")
	flag.Parse()

	pkgs, err := resolvePkgs(context.Background(), flag.Args())
	if err != nil {
		return err
	}

	data, err := runBenchmarks(context.Background(), pkgs, *benchtime, *parallel, *cores)
	if err != nil {
		return err
	}

	bs, e, has := parse(data)
	if !has {
		fmt.Println("No benchmarks found")
		return nil
	}

	pairs, unpaired := pairUp(bs)

	type group struct {
		pairs    []pair
		unpaired []bench
	}
	gm := map[string]*group{}
	var pkgOrder []string
	for _, p := range pairs {
		pkg := p.ours.pkg
		if _, ok := gm[pkg]; !ok {
			gm[pkg] = &group{}
			pkgOrder = append(pkgOrder, pkg)
		}
		gm[pkg].pairs = append(gm[pkg].pairs, p)
	}
	for _, b := range unpaired {
		if _, ok := gm[b.pkg]; !ok {
			gm[b.pkg] = &group{}
			pkgOrder = append(pkgOrder, b.pkg)
		}
		gm[b.pkg].unpaired = append(gm[b.pkg].unpaired, b)
	}
	sort.Strings(pkgOrder)
	for _, pkg := range pkgOrder {
		g := gm[pkg]
		printTable(pkg, g.pairs, g.unpaired)
	}

	printFooter(e)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
