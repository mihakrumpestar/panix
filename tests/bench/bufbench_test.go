package main

import (
	"bytes"
	"math/bits"
	"sync"
	"testing"
)

// ─── Test data ──────────────────────────────────────────────────────────────

var benchLineData = func() [][]byte {
	raw := []string{
		"\x1b[38;2;180;190;254m╰── \x1b[0m" +
			"\x1b[38;2;166;173;200m⚙\x1b[0m" +
			"\x1b[38;2;180;190;254m build-nixos-system\x1b[0m " +
			"\x1b[38;2;137;180;250msucceeded\x1b[0m   (12.34s)",
		"\x1b[38;2;180;190;254m├── \x1b[0m" +
			"\x1b[38;2;166;173;200m✔\x1b[0m" +
			"\x1b[38;2;166;227;161m nixosConfigurations\x1b[0m " +
			"\x1b[38;2;204;120;50m⚙\x1b[0m laptop           (45.67s)",
		"\x1b[38;2;180;190;254m│   \x1b[0m" +
			"\x1b[38;2;180;190;254m├── \x1b[0m" +
			"\x1b[38;2;137;180;250m⠋\x1b[0m" +
			"\x1b[38;2;205;214;244m build\x1b[0m" +
			"                                     (2m31s)",
		"\x1b[38;2;180;190;254m│   \x1b[0m" +
			"\x1b[38;2;180;190;254m╰── \x1b[0m" +
			"\x1b[38;2;166;227;161m✔\x1b[0m" +
			"\x1b[38;2;205;214;244m test\x1b[0m" +
			"                                      (8.12s)",
		"\x1b[38;2;180;190;254m│       \x1b[0m" +
			"\x1b[38;2;166;173;200m❯\x1b[0m" +
			"\x1b[38;2;205;214;244m nix build " +
			".#nixosConfigurations.laptop.config.system.build.toplevel\x1b[0m",
		"\x1b[38;2;180;190;254m╰── \x1b[0m" +
			"\x1b[38;2;166;173;200m✔\x1b[0m" +
			"\x1b[38;2;166;227;161m homeConfigurations\x1b[0m " +
			"\x1b[38;2;204;120;50m⚙\x1b[0m laptop           (23.45s)",
		"╭──────────────────────────────────────" +
			"────────────────────────────────────────╮",
		"│ \x1b[38;2;180;190;254mFlake\x1b[0m  " +
			"\x1b[38;2;166;227;161m✔\x1b[0m  " +
			"\x1b[38;2;137;180;250mnixos-config\x1b[0m" +
			"                                                        │",
		"╰──────────────────────────────────────" +
			"────────────────────────────────────────╯",
	}

	lines := make([][]byte, len(raw))
	for i, s := range raw {
		lines[i] = []byte(s)
	}

	return lines
}()

const benchNumLines = 80

var minimalLine = []byte("build-nixos succeeded")

var benchPrevLines = func() [][]byte {
	lines := make([][]byte, benchNumLines)
	for i := range lines {
		src := benchLineData[i%len(benchLineData)]
		cp := make([]byte, len(src))
		copy(cp, src)
		lines[i] = cp
	}

	for i := 4; i < len(lines); i += 5 {
		lines[i] = []byte(
			"\x1b[38;2;180;190;254m│   \x1b[0m" +
				"\x1b[38;2;137;180;250m⠹\x1b[0m" +
				"\x1b[38;2;205;214;244m build\x1b[0m" +
				"                                     (2m32s)")
	}

	return lines
}()

// ─── Interface ──────────────────────────────────────────────────────────────

type benchBuffer interface {
	WriteLine(line []byte)
	OverrideLastLine(line []byte)
	Reset()
	Diff(old benchBuffer) []int
}

func writeAllLines(buf benchBuffer) {
	for i := range benchNumLines {
		buf.WriteLine(benchLineData[i%len(benchLineData)])
	}
}

// ─── ByteSliceSlice ─────────────────────────────────────────────────────────

type byteSliceSlice struct{ lines [][]byte }

func (b *byteSliceSlice) WriteLine(line []byte) {
	if len(b.lines) < cap(b.lines) {
		idx := len(b.lines)
		b.lines = b.lines[:idx+1]
		b.lines[idx] = append(b.lines[idx][:0], line...)
	} else {
		capacity := max(len(line)*2, 256)
		newLine := make([]byte, 0, capacity)
		newLine = append(newLine, line...)
		b.lines = append(b.lines, newLine)
	}
}

func (b *byteSliceSlice) Reset() {
	for i := range b.lines {
		b.lines[i] = b.lines[i][:0]
	}

	b.lines = b.lines[:0]
}

func (b *byteSliceSlice) OverrideLastLine(line []byte) {
	if len(b.lines) == 0 {
		b.WriteLine(line)

		return
	}

	b.lines[len(b.lines)-1] = append(b.lines[len(b.lines)-1][:0], line...)
}

//nolint:forcetypeassert // type guaranteed by construction
func (b *byteSliceSlice) Diff(old benchBuffer) []int {
	other := old.(*byteSliceSlice)
	commonLen := min(len(b.lines), len(other.lines))

	var diffs []int

	for idx := range commonLen {
		if !bytes.Equal(b.lines[idx], other.lines[idx]) {
			diffs = append(diffs, idx)
		}
	}

	for idx := commonLen; idx < len(b.lines); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

// ─── PooledByteSlice ────────────────────────────────────────────────────────

var pooledByteSlicePool = sync.Pool{
	New: func() any {
		return &byteSliceSlice{lines: make([][]byte, 0, benchNumLines)}
	},
}

// ─── StringSlice ────────────────────────────────────────────────────────────

type stringSlice struct{ lines []string }

func (s *stringSlice) WriteLine(line []byte) {
	if len(s.lines) < cap(s.lines) {
		idx := len(s.lines)
		s.lines = s.lines[:idx+1]
		s.lines[idx] = string(line)
	} else {
		s.lines = append(s.lines, string(line))
	}
}

func (s *stringSlice) Reset() {
	for i := range s.lines {
		s.lines[i] = ""
	}

	s.lines = s.lines[:0]
}

func (s *stringSlice) OverrideLastLine(line []byte) {
	if len(s.lines) == 0 {
		s.WriteLine(line)

		return
	}

	s.lines[len(s.lines)-1] = string(line)
}

//nolint:forcetypeassert // type guaranteed by construction
func (s *stringSlice) Diff(old benchBuffer) []int {
	other := old.(*stringSlice)
	commonLen := min(len(s.lines), len(other.lines))

	var diffs []int

	for idx := range commonLen {
		if s.lines[idx] != other.lines[idx] {
			diffs = append(diffs, idx)
		}
	}

	for idx := commonLen; idx < len(s.lines); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

// ─── ContiguousRefBuffer ────────────────────────────────────────────────────

type lineRef struct{ off, len_ uint32 }

func diffLineRefs(dataA []byte, refsA []lineRef, dataB []byte, refsB []lineRef) []int {
	commonLen := min(len(refsA), len(refsB))

	var diffs []int

	for idx := range commonLen {
		nr, or_ := refsA[idx], refsB[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(dataA[nr.off:nr.off+nr.len_], dataB[or_.off:or_.off+or_.len_]) {
			continue
		}

		diffs = append(diffs, idx)
	}

	for idx := commonLen; idx < len(refsA); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

type contiguousRefBuffer struct {
	data []byte
	refs []lineRef
}

//nolint:gosec // benchmark code, sizes are controlled
func (a *contiguousRefBuffer) WriteLine(line []byte) {
	off := uint32(len(a.data))
	a.data = append(a.data, line...)
	a.refs = append(a.refs, lineRef{off, uint32(len(line))})
}

func (a *contiguousRefBuffer) Reset() {
	a.data = a.data[:0]
	a.refs = a.refs[:0]
}

//nolint:gosec // benchmark code, sizes are controlled
func (a *contiguousRefBuffer) OverrideLastLine(line []byte) {
	if len(a.refs) == 0 {
		a.WriteLine(line)

		return
	}

	last := &a.refs[len(a.refs)-1]
	a.data = a.data[:last.off]
	a.data = append(a.data, line...)
	last.len_ = uint32(len(line))
}

//nolint:forcetypeassert // type guaranteed by construction
func (a *contiguousRefBuffer) Diff(old benchBuffer) []int {
	return diffLineRefs(a.data, a.refs, old.(*contiguousRefBuffer).data, old.(*contiguousRefBuffer).refs)
}

var contiguousRefPool = sync.Pool{
	New: func() any {
		return &contiguousRefBuffer{
			data: make([]byte, 0, 16*1024),
			refs: make([]lineRef, 0, benchNumLines),
		}
	},
}

// ─── BytesBufferSlice ──────────────────────────────────────────────────────

type bytesBufferSlice struct{ buffers []bytes.Buffer }

//nolint:gosec // benchmark code, sizes are controlled
func (b *bytesBufferSlice) WriteLine(line []byte) {
	if len(b.buffers) < cap(b.buffers) {
		idx := len(b.buffers)
		b.buffers = b.buffers[:idx+1]
		b.buffers[idx].Reset()
		b.buffers[idx].Write(line)
	} else {
		var buf bytes.Buffer
		buf.Grow(max(len(line)*2, 256))
		buf.Write(line)
		b.buffers = append(b.buffers, buf)
	}
}

func (b *bytesBufferSlice) Reset() {
	for i := range b.buffers {
		b.buffers[i].Reset()
	}

	b.buffers = b.buffers[:0]
}

func (b *bytesBufferSlice) OverrideLastLine(line []byte) {
	if len(b.buffers) == 0 {
		b.WriteLine(line)

		return
	}

	last := &b.buffers[len(b.buffers)-1]
	last.Reset()
	last.Write(line)
}

//nolint:forcetypeassert // type guaranteed by construction
func (b *bytesBufferSlice) Diff(old benchBuffer) []int {
	other := old.(*bytesBufferSlice)
	commonLen := min(len(b.buffers), len(other.buffers))

	var diffs []int

	for idx := range commonLen {
		if !bytes.Equal(b.buffers[idx].Bytes(), other.buffers[idx].Bytes()) {
			diffs = append(diffs, idx)
		}
	}

	for idx := commonLen; idx < len(b.buffers); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

var bytesBufferPool = sync.Pool{
	New: func() any {
		return &bytesBufferSlice{buffers: make([]bytes.Buffer, 0, benchNumLines)}
	},
}

// ─── AtomicContiguousRef (mutex-guarded ContiguousRef) ────────────────────────

type atomicContiguousRef struct {
	mu  sync.Mutex
	buf contiguousRefBuffer
}

func (a *atomicContiguousRef) WriteLine(line []byte) {
	a.mu.Lock()
	a.buf.WriteLine(line)
	a.mu.Unlock()
}

func (a *atomicContiguousRef) OverrideLastLine(line []byte) {
	a.mu.Lock()
	a.buf.OverrideLastLine(line)
	a.mu.Unlock()
}

func (a *atomicContiguousRef) Reset() {
	a.mu.Lock()
	a.buf.Reset()
	a.mu.Unlock()
}

//nolint:forcetypeassert // type guaranteed by construction
func (a *atomicContiguousRef) Diff(old benchBuffer) []int {
	other := old.(*atomicContiguousRef)

	a.mu.Lock()
	other.mu.Lock()
	defer a.mu.Unlock()
	defer other.mu.Unlock()

	return a.buf.Diff(&other.buf)
}

// ─── OptionalMutexContiguousRef (bool-gated mutex on ContiguousRef) ─────────

type optionalMutexContiguousRef struct {
	mu         sync.Mutex
	buf        contiguousRefBuffer
	threadSafe bool
}

func (o *optionalMutexContiguousRef) WriteLine(line []byte) {
	if o.threadSafe {
		o.mu.Lock()
	}

	o.buf.WriteLine(line)

	if o.threadSafe {
		o.mu.Unlock()
	}
}

func (o *optionalMutexContiguousRef) OverrideLastLine(line []byte) {
	if o.threadSafe {
		o.mu.Lock()
	}

	o.buf.OverrideLastLine(line)

	if o.threadSafe {
		o.mu.Unlock()
	}
}

func (o *optionalMutexContiguousRef) Reset() {
	if o.threadSafe {
		o.mu.Lock()
	}

	o.buf.Reset()

	if o.threadSafe {
		o.mu.Unlock()
	}
}

//nolint:forcetypeassert // type guaranteed by construction
func (o *optionalMutexContiguousRef) Diff(old benchBuffer) []int {
	otherOpt := old.(*optionalMutexContiguousRef)

	if o.threadSafe {
		o.mu.Lock()
		otherOpt.mu.Lock()
		defer o.mu.Unlock()
		defer otherOpt.mu.Unlock()
	}

	return o.buf.Diff(&otherOpt.buf)
}

// ─── BitmaskDiffRef (ContiguousRef + uint64 bitset instead of []int) ─────────

type bitmaskDiffRef struct {
	data []byte
	refs []lineRef
	bits []uint64
}

//nolint:gosec // benchmark code, sizes are controlled
func (b *bitmaskDiffRef) WriteLine(line []byte) {
	off := uint32(len(b.data))
	b.data = append(b.data, line...)
	b.refs = append(b.refs, lineRef{off, uint32(len(line))})
}

//nolint:gosec // benchmark code, sizes are controlled
func (b *bitmaskDiffRef) OverrideLastLine(line []byte) {
	if len(b.refs) == 0 {
		b.WriteLine(line)

		return
	}

	last := &b.refs[len(b.refs)-1]
	b.data = b.data[:last.off]
	b.data = append(b.data, line...)
	last.len_ = uint32(len(line))
}

func (b *bitmaskDiffRef) Reset() {
	b.data = b.data[:0]

	b.refs = b.refs[:0]
	for i := range b.bits {
		b.bits[i] = 0
	}
}

//nolint:forcetypeassert // type guaranteed by construction
func (b *bitmaskDiffRef) Diff(old benchBuffer) []int {
	other := old.(*bitmaskDiffRef)
	commonLen := min(len(b.refs), len(other.refs))

	nWords := (max(len(b.refs), len(other.refs)) + 63) / 64
	if nWords > len(b.bits) {
		b.bits = make([]uint64, nWords)
	}

	for i := range b.bits {
		b.bits[i] = 0
	}

	for idx := range commonLen {
		nr, or_ := b.refs[idx], other.refs[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(b.data[nr.off:nr.off+nr.len_], other.data[or_.off:or_.off+or_.len_]) {
			continue
		}

		b.bits[idx/64] |= 1 << (idx % 64)
	}

	for idx := commonLen; idx < len(b.refs); idx++ {
		b.bits[idx/64] |= 1 << (idx % 64)
	}
	// Convert bitset to []int for interface compatibility
	var diffs []int

	for wordIdx, word := range b.bits[:nWords] {
		for word != 0 {
			low := word &^ (word - 1)
			bit := bits.TrailingZeros64(low)
			diffs = append(diffs, wordIdx*64+bit)
			word &^= low
		}
	}

	return diffs
}

// ─── PreAllocDiffRef (ContiguousRef + pre-allocated diffs slice) ────────────

type preAllocDiffRef struct {
	data  []byte
	refs  []lineRef
	diffs []int
}

//nolint:gosec // benchmark code, sizes are controlled
func (p *preAllocDiffRef) WriteLine(line []byte) {
	off := uint32(len(p.data))
	p.data = append(p.data, line...)
	p.refs = append(p.refs, lineRef{off, uint32(len(line))})
}

//nolint:gosec // benchmark code, sizes are controlled
func (p *preAllocDiffRef) OverrideLastLine(line []byte) {
	if len(p.refs) == 0 {
		p.WriteLine(line)

		return
	}

	last := &p.refs[len(p.refs)-1]
	p.data = p.data[:last.off]
	p.data = append(p.data, line...)
	last.len_ = uint32(len(line))
}

func (p *preAllocDiffRef) Reset() {
	p.data = p.data[:0]
	p.refs = p.refs[:0]
}

//nolint:forcetypeassert // type guaranteed by construction
func (p *preAllocDiffRef) Diff(old benchBuffer) []int {
	other := old.(*preAllocDiffRef)
	commonLen := min(len(p.refs), len(other.refs))

	p.diffs = p.diffs[:0]
	for idx := range commonLen {
		nr, or_ := p.refs[idx], other.refs[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(p.data[nr.off:nr.off+nr.len_], other.data[or_.off:or_.off+or_.len_]) {
			continue
		}

		p.diffs = append(p.diffs, idx)
	}

	for idx := commonLen; idx < len(p.refs); idx++ {
		p.diffs = append(p.diffs, idx)
	}

	return p.diffs
}

var preAllocDiffRefPool = sync.Pool{
	New: func() any {
		return &preAllocDiffRef{
			data:  make([]byte, 0, 16*1024),
			refs:  make([]lineRef, 0, benchNumLines),
			diffs: make([]int, 0, benchNumLines),
		}
	},
}

// ─── DoubleBufRef (double-buffered ContiguousRef with swap) ─────────────────

type doubleBufRef struct {
	bufs [2]contiguousRefBuffer
	cur  int
}

//nolint:funcorder // benchmark code, ordering by feature
func (d *doubleBufRef) active() *contiguousRefBuffer { return &d.bufs[d.cur] }

func (d *doubleBufRef) WriteLine(line []byte)        { d.active().WriteLine(line) }
func (d *doubleBufRef) OverrideLastLine(line []byte) { d.active().OverrideLastLine(line) }

func (d *doubleBufRef) Reset() {
	// Swap: current becomes previous, previous gets cleared and becomes new current
	d.cur = 1 - d.cur
	d.active().data = d.active().data[:0]
	d.active().refs = d.active().refs[:0]
}

//nolint:forcetypeassert // type guaranteed by construction
func (d *doubleBufRef) Diff(old benchBuffer) []int {
	o := old.(*doubleBufRef)

	return d.active().Diff(o.active())
}

// ─── InlineShortRef (lines ≤24 bytes stored inline in ref struct) ───────────

const inlineThreshold = 24

type inlineLineRef struct {
	off    uint32
	len_   uint16
	inline bool
	buf    [inlineThreshold]byte
}

type inlineShortRef struct {
	data []byte
	refs []inlineLineRef
}

//nolint:gosec // benchmark code, sizes are controlled
func (i *inlineShortRef) WriteLine(line []byte) {
	if len(line) <= inlineThreshold {
		ref := inlineLineRef{len_: uint16(len(line)), inline: true}
		copy(ref.buf[:], line)
		i.refs = append(i.refs, ref)
	} else {
		off := uint32(len(i.data))
		i.data = append(i.data, line...)
		i.refs = append(i.refs, inlineLineRef{off: off, len_: uint16(len(line))})
	}
}

//nolint:gosec // benchmark code, sizes are controlled
func (i *inlineShortRef) OverrideLastLine(line []byte) {
	if len(i.refs) == 0 {
		i.WriteLine(line)

		return
	}

	last := &i.refs[len(i.refs)-1]
	if !last.inline {
		i.data = i.data[:last.off]
	}

	if len(line) <= inlineThreshold {
		last.inline = true
		copy(last.buf[:], line)
		last.len_ = uint16(len(line))
	} else {
		off := uint32(len(i.data))
		i.data = append(i.data, line...)
		last.off = off
		last.len_ = uint16(len(line))
		last.inline = false
	}
}

func (i *inlineShortRef) Reset() {
	i.data = i.data[:0]
	i.refs = i.refs[:0]
}

//nolint:funcorder // benchmark code, ordering by feature
func (i *inlineShortRef) lineData(ref *inlineLineRef) []byte {
	if ref.inline {
		return ref.buf[:ref.len_]
	}

	return i.data[ref.off : ref.off+uint32(ref.len_)]
}

//nolint:forcetypeassert // type guaranteed by construction
func (i *inlineShortRef) Diff(old benchBuffer) []int {
	other := old.(*inlineShortRef)
	commonLen := min(len(i.refs), len(other.refs))

	var diffs []int

	for idx := range commonLen {
		nr, or_ := &i.refs[idx], &other.refs[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(i.lineData(nr), other.lineData(or_)) {
			continue
		}

		diffs = append(diffs, idx)
	}

	for idx := commonLen; idx < len(i.refs); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

// ─── ArenaResetRef (ContiguousRef, Reset only truncates refs, data grows) ───

type arenaResetRef struct {
	data []byte
	refs []lineRef
}

//nolint:gosec // benchmark code, sizes are controlled
func (a *arenaResetRef) WriteLine(line []byte) {
	off := uint32(len(a.data))
	a.data = append(a.data, line...)
	a.refs = append(a.refs, lineRef{off, uint32(len(line))})
}

//nolint:gosec // benchmark code, sizes are controlled
func (a *arenaResetRef) OverrideLastLine(line []byte) {
	if len(a.refs) == 0 {
		a.WriteLine(line)

		return
	}

	last := &a.refs[len(a.refs)-1]
	a.data = a.data[:last.off]
	a.data = append(a.data, line...)
	last.len_ = uint32(len(line))
}

func (a *arenaResetRef) Reset() {
	// Only reset refs; data stays allocated, WriteLine overwrites from start
	a.refs = a.refs[:0]
	a.data = a.data[:0]
}

//nolint:forcetypeassert // type guaranteed by construction
func (a *arenaResetRef) Diff(old benchBuffer) []int {
	return diffLineRefs(a.data, a.refs, old.(*arenaResetRef).data, old.(*arenaResetRef).refs)
}

// ─── BlobDiffRef (ContiguousRef + whole-blob memcmp fast path) ───────────────

type blobDiffRef struct {
	data []byte
	refs []lineRef
}

//nolint:gosec // benchmark code, sizes are controlled
func (b *blobDiffRef) WriteLine(line []byte) {
	off := uint32(len(b.data))
	b.data = append(b.data, line...)
	b.refs = append(b.refs, lineRef{off, uint32(len(line))})
}

//nolint:gosec // benchmark code, sizes are controlled
func (b *blobDiffRef) OverrideLastLine(line []byte) {
	if len(b.refs) == 0 {
		b.WriteLine(line)

		return
	}

	last := &b.refs[len(b.refs)-1]
	b.data = b.data[:last.off]
	b.data = append(b.data, line...)
	last.len_ = uint32(len(line))
}

func (b *blobDiffRef) Reset() {
	b.data = b.data[:0]
	b.refs = b.refs[:0]
}

//nolint:forcetypeassert // diff is inherently branchy; type guaranteed by construction
func (b *blobDiffRef) Diff(old benchBuffer) []int {
	other := old.(*blobDiffRef)
	commonLen := min(len(b.refs), len(other.refs))

	// Fast path: if all line lengths match and total data length matches,
	// do a single memcmp on the whole blob
	if len(b.data) == len(other.data) && len(b.refs) == len(other.refs) {
		allMatch := true

		for idx := range commonLen {
			if b.refs[idx].len_ != other.refs[idx].len_ {
				allMatch = false

				break
			}
		}

		if allMatch && bytes.Equal(b.data, other.data) {
			return nil
		}
	}

	var diffs []int

	for idx := range commonLen {
		nr, or_ := b.refs[idx], other.refs[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(b.data[nr.off:nr.off+nr.len_], other.data[or_.off:or_.off+or_.len_]) {
			continue
		}

		diffs = append(diffs, idx)
	}

	for idx := commonLen; idx < len(b.refs); idx++ {
		diffs = append(diffs, idx)
	}

	return diffs
}

// ─── Registry ───────────────────────────────────────────────────────────────

type bufType struct {
	name    string
	new     func() benchBuffer
	pool    *sync.Pool
	hasPool bool
}

var bufTypes = []bufType{
	{
		name:    "PooledContiguousRef",
		new:     func() benchBuffer { return contiguousRefPool.Get().(*contiguousRefBuffer) }, //nolint:forcetypeassert // pool type guaranteed
		pool:    &contiguousRefPool,
		hasPool: true,
	},
	{
		name: "ByteSliceSlice",
		new:  func() benchBuffer { return &byteSliceSlice{lines: make([][]byte, 0, benchNumLines)} },
	},
	{
		name: "StringSlice",
		new:  func() benchBuffer { return &stringSlice{lines: make([]string, 0, benchNumLines)} },
	},
	{
		name:    "PooledByteSlice",
		new:     func() benchBuffer { return pooledByteSlicePool.Get().(*byteSliceSlice) }, //nolint:forcetypeassert // pool type guaranteed
		pool:    &pooledByteSlicePool,
		hasPool: true,
	},
	{
		name: "ContiguousRef",
		new: func() benchBuffer {
			return &contiguousRefBuffer{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)}
		},
	},
	/*

			Feature
		──────────────────
		DiffLines
		OverrideLastLine
		Reset
		WriteAndDiff
		WriteLine
		WriteOverrideDiff

		│          ChunkedRef │
		┼─────────────────────┼
		│   646 ns (2) +1.05× │
		│    24 ns (0) +2.86× │
		│   3.5 µs (4) +4.61× │
		│   3.9 µs (6) +2.69× │
		│   2.9 µs (4) +3.84× │
		│   4.5 µs (7) +3.21× │

		{
			name:    "ChunkedRef",
			new:     func() benchBuffer { return &chunkedRefBuffer{} },
			pool:    &chunkedRefPool,
			hasPool: true,
		},
	*/
	/*

		│           LineStruct │
		┼──────────────────────┼
		│    584 ns (2) −1.06× │
		│    150 ns (0) +18.0× │
		│   12.5 µs (0) +16.3× │
		│   13.2 µs (2) +9.13× │
		│   12.7 µs (0) +16.6× │
		│   13.1 µs (2) +9.26× │

		{
			name:    "LineStruct",
			new:     func() benchBuffer { return &lineStructBuffer{lines: make([]lineStruct, 0, benchNumLines)} },
			pool:    &lineStructPool,
			hasPool: true,
		},
	*/
	/*

		│       StringsBuilder │
		┼──────────────────────┼
		│    575 ns (2) −1.07× │
		│     54 ns (1) +6.52× │
		│   4.7 µs (80) +6.07× │
		│   5.6 µs (82) +3.89× │
		│   4.6 µs (80) +6.09× │
		│   5.7 µs (83) +4.00× │

		{
			name:    "StringsBuilder",
			new:     func() benchBuffer { return &stringsBuilderSlice{builders: make([]strings.Builder, 0, benchNumLines)} },
			pool:    &stringsBuilderPool,
			hasPool: true,
		},
	*/
	{
		name:    "BytesBuffer",
		new:     func() benchBuffer { return &bytesBufferSlice{buffers: make([]bytes.Buffer, 0, benchNumLines)} },
		pool:    &bytesBufferPool,
		hasPool: true,
	},
	/*

		│    FlatStringsBuilder │
		┼───────────────────────┼
		│     662 ns (2) +1.07× │
		│      1.3 µs (1) +154× │
		│    9.7 µs (13) +12.7× │
		│   10.7 µs (15) +7.38× │
		│    9.7 µs (13) +12.7× │
		│   12.6 µs (16) +8.90× │

		{
			name:    "FlatStringsBuilder",
			new:     func() benchBuffer { return &flatStringsBuilder{offsets: make([]int, 0, benchNumLines)} },
			pool:    &flatStringsBuilderPool,
			hasPool: true,
		},

	*/
	/*
		│      UnsafeByteSlice │
		┼──────────────────────┼
		│    565 ns (2) −1.09× │
		│     47 ns (1) +5.60× │
		│   3.8 µs (80) +4.98× │
		│   4.8 µs (82) +3.33× │
		│   3.9 µs (80) +5.07× │
		│   4.8 µs (83) +3.40× │

		{
			name: "UnsafeByteSlice",
			new:  func() benchBuffer { return &unsafeByteSlice{lines: make([]string, 0, benchNumLines)} },
		},
	*/
	/*

		│    UnsafeContiguousRef │
		┼────────────────────────┼
		│      653 ns (2) +1.06× │
		│       1.7 µs (2) +199× │
		│    92.2 µs (159) +120× │
		│   90.3 µs (161) +62.5× │
		│    85.9 µs (159) +113× │
		│   96.5 µs (163) +68.2× │

		{
			name: "UnsafeContiguousRef",
			new: func() benchBuffer {
				return &unsafeContiguousRef{refs: make([]lineRef, 0, benchNumLines)}
			},
		},
	*/
	{
		name: "AtomicContiguousRef",
		new: func() benchBuffer {
			return &atomicContiguousRef{buf: contiguousRefBuffer{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)}}
		},
	},
	{
		name: "OptMutexRef_Unsafe",
		new: func() benchBuffer {
			return &optionalMutexContiguousRef{
				buf: contiguousRefBuffer{
					data: make([]byte, 0, 16*1024),
					refs: make([]lineRef, 0, benchNumLines),
				},
			}
		},
	},
	{
		name: "OptMutexRef_Safe",
		new: func() benchBuffer {
			return &optionalMutexContiguousRef{
				buf: contiguousRefBuffer{
					data: make([]byte, 0, 16*1024),
					refs: make([]lineRef, 0, benchNumLines),
				},
				threadSafe: true,
			}
		},
	},
	{
		name: "BitmaskDiffRef",
		new: func() benchBuffer {
			return &bitmaskDiffRef{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines), bits: make([]uint64, (benchNumLines+63)/64)}
		},
	},
	{
		name: "PreAllocDiffRef",
		new: func() benchBuffer {
			return &preAllocDiffRef{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines), diffs: make([]int, 0, benchNumLines)}
		},
	},
	{
		name:    "PooledPreAllocDiffRef",
		new:     func() benchBuffer { return preAllocDiffRefPool.Get().(*preAllocDiffRef) }, //nolint:forcetypeassert // pool type guaranteed
		pool:    &preAllocDiffRefPool,
		hasPool: true,
	},
	{
		name: "DoubleBufRef",
		new: func() benchBuffer {
			return &doubleBufRef{
				bufs: [2]contiguousRefBuffer{
					{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)},
					{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)},
				},
			}
		},
	},
	{
		name: "InlineShortRef",
		new: func() benchBuffer {
			return &inlineShortRef{data: make([]byte, 0, 16*1024), refs: make([]inlineLineRef, 0, benchNumLines)}
		},
	},
	{
		name: "ArenaResetRef",
		new: func() benchBuffer {
			return &arenaResetRef{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)}
		},
	},
	{
		name: "BlobDiffRef",
		new: func() benchBuffer {
			return &blobDiffRef{data: make([]byte, 0, 16*1024), refs: make([]lineRef, 0, benchNumLines)}
		},
	},
}

// ─── Helpers for old-frame data ─────────────────────────────────────────────

func makeOldBuffer(benchType *bufType) benchBuffer {
	buf := benchType.new()
	for i := range benchNumLines {
		buf.WriteLine(benchPrevLines[i])
	}

	return buf
}

func putPool(benchType *bufType, buf benchBuffer) {
	if benchType.hasPool {
		benchType.pool.Put(buf)
	}
}

// ─── Scenarios ──────────────────────────────────────────────────────────────

func benchWriteLine(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	for b.Loop() {
		buf.Reset()
		writeAllLines(buf)
	}

	putPool(benchType, buf)
}

func benchDiffLines(b *testing.B, benchType *bufType) {
	b.Helper()

	newBuf := benchType.new()
	writeAllLines(newBuf)

	oldBuf := makeOldBuffer(benchType)

	for b.Loop() {
		newBuf.Diff(oldBuf)
	}

	putPool(benchType, newBuf)
	putPool(benchType, oldBuf)
}

func benchWriteAndDiff(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	oldBuf := makeOldBuffer(benchType)

	for b.Loop() {
		buf.Reset()
		writeAllLines(buf)
		buf.Diff(oldBuf)
	}

	putPool(benchType, buf)
	putPool(benchType, oldBuf)
}

func benchReset(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	writeAllLines(buf)

	for b.Loop() {
		buf.Reset()
		writeAllLines(buf)
	}

	putPool(benchType, buf)
}

func benchOverrideLastLine(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	writeAllLines(buf)

	overrideLine := benchLineData[3]

	for b.Loop() {
		buf.OverrideLastLine(overrideLine)
	}

	putPool(benchType, buf)
}

func benchWriteOverrideDiff(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	oldBuf := makeOldBuffer(benchType)
	overrideLine := benchLineData[3]

	for b.Loop() {
		buf.Reset()
		writeAllLines(buf)
		buf.OverrideLastLine(overrideLine)
		buf.Diff(oldBuf)
	}

	putPool(benchType, buf)
	putPool(benchType, oldBuf)
}

// ─── Top-level benchmark ────────────────────────────────────────────────────

//nolint:dupl // benchmark structure is inherently similar
func BenchmarkBufTypes(b *testing.B) {
	for i := range bufTypes {
		benchType := &bufTypes[i]
		b.Run(benchType.name+"__WriteLine", func(b *testing.B) { benchWriteLine(b, benchType) })
		b.Run(benchType.name+"__DiffLines", func(b *testing.B) { benchDiffLines(b, benchType) })
		b.Run(benchType.name+"__WriteAndDiff", func(b *testing.B) { benchWriteAndDiff(b, benchType) })
		b.Run(benchType.name+"__Reset", func(b *testing.B) { benchReset(b, benchType) })
		b.Run(benchType.name+"__OverrideLastLine", func(b *testing.B) { benchOverrideLastLine(b, benchType) })
		b.Run(benchType.name+"__WriteOverrideDiff", func(b *testing.B) { benchWriteOverrideDiff(b, benchType) })
	}
}

// ─── Minimal (single short line) benchmarks ──────────────────────────────────

func benchMinimalWriteLine(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	for b.Loop() {
		buf.Reset()
		buf.WriteLine(minimalLine)
	}

	putPool(benchType, buf)
}

func benchMinimalDiffLines(b *testing.B, benchType *bufType) {
	b.Helper()

	newBuf := benchType.new()
	newBuf.WriteLine(minimalLine)

	oldBuf := benchType.new()
	oldBuf.WriteLine(minimalLine)

	for b.Loop() {
		newBuf.Diff(oldBuf)
	}

	putPool(benchType, newBuf)
	putPool(benchType, oldBuf)
}

func benchMinimalWriteAndDiff(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	oldBuf := benchType.new()
	oldBuf.WriteLine(minimalLine)

	for b.Loop() {
		buf.Reset()
		buf.WriteLine(minimalLine)
		buf.Diff(oldBuf)
	}

	putPool(benchType, buf)
	putPool(benchType, oldBuf)
}

func benchMinimalReset(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	buf.WriteLine(minimalLine)

	for b.Loop() {
		buf.Reset()
		buf.WriteLine(minimalLine)
	}

	putPool(benchType, buf)
}

func benchMinimalOverrideLastLine(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	buf.WriteLine(minimalLine)

	for b.Loop() {
		buf.OverrideLastLine(minimalLine)
	}

	putPool(benchType, buf)
}

func benchMinimalWriteOverrideDiff(b *testing.B, benchType *bufType) {
	b.Helper()

	buf := benchType.new()
	oldBuf := benchType.new()
	oldBuf.WriteLine(minimalLine)

	for b.Loop() {
		buf.Reset()
		buf.WriteLine(minimalLine)
		buf.OverrideLastLine(minimalLine)
		buf.Diff(oldBuf)
	}

	putPool(benchType, buf)
	putPool(benchType, oldBuf)
}

//nolint:dupl // benchmark structure is inherently similar
func BenchmarkBufTypesMinimal(b *testing.B) {
	for i := range bufTypes {
		benchType := &bufTypes[i]
		b.Run(benchType.name+"__MinimalWriteLine", func(b *testing.B) { benchMinimalWriteLine(b, benchType) })
		b.Run(benchType.name+"__MinimalDiffLines", func(b *testing.B) { benchMinimalDiffLines(b, benchType) })
		b.Run(benchType.name+"__MinimalWriteAndDiff", func(b *testing.B) { benchMinimalWriteAndDiff(b, benchType) })
		b.Run(benchType.name+"__MinimalReset", func(b *testing.B) { benchMinimalReset(b, benchType) })
		b.Run(benchType.name+"__MinimalOverrideLastLine", func(b *testing.B) { benchMinimalOverrideLastLine(b, benchType) })
		b.Run(benchType.name+"__MinimalWriteOverrideDiff", func(b *testing.B) { benchMinimalWriteOverrideDiff(b, benchType) })
	}
}
