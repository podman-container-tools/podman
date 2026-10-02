// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import (
	"bytes"
	"fmt"
	"go/types"
	"io"
	"io/fs"
	"path"

	"golang.org/x/tools/go/gcexportdata"
)

// Reading a package from the compiler's export data.
//
// Type-checking the standard library from source takes nearly all the time of a full scan:
// ~190 packages and ~1195 files for a fixture as small as the petstore, and a WebAssembly guest pays
// a five to six-fold compute tax on top.
//
// None of that work is discovery — the answers were already computed when the toolchain built those packages,
// and the compiler wrote them down. So we read them instead.
//
// The saving is in the parsing and type-checking avoided, not in the I/O. When executing inside a WASI host,
// filesystem syscalls during a scan account for less than 2%.

// exportedPackage builds the *Package a dependency served from export data is seen as.
//
// Types only, and no syntax: a package reached this way is one that says nothing about its own types — see
// carriesAnnotations — so the load has no reason to read it.
//
// It is told WHERE its source is even so. Saying nothing about its own types is not the same as declaring nothing:
// the scanned code can name a type from here and want the declaration for it, and the file list lets
// [Loader.ReadBackSource] answer that later without resolving the import a second time. The marker scan has just
// resolved it, so this costs a slice.
func (ld *loadState) exportedPackage(importPath string, tpkg *types.Package) *Package {
	return &Package{
		ID:      importPath,
		Name:    tpkg.Name(),
		PkgPath: importPath,
		GoFiles: ld.sourceFiles(importPath),
		Types:   tpkg,
		Fset:    ld.fset,
		Imports: map[string]*Package{},
	}
}

// carriesAnnotations reports whether a dependency's source says anything a scan would read.
//
// This is the whole of the export-data policy, and it is a policy rather than an optimisation.
//
// Export data holds types and not comments, so the choice is made per package and made whole: a dependency that
// carries annotations is loaded from source like any other, and one that does not is taken from export data with no
// syntax at all.
//
// Nothing is lost either way, and the saving survives, because it was never in the packages a scan reads — it is in
// the closure behind them, which export data still serves.
//
// The middle option — export-data types with parsed syntax bolted on beside them — was once impossible, and that is
// no longer why we avoid it. The builders used to read types.Info.Types, whose entries cannot be constructed outside
// go/types (the field distinguishing a type from a value is unexported), so a package assembled that way handed them
// declarations with no record of what those declarations denote: not a quieter scan but a panicking one. They have
// since been taken off that map — the one reader left is a documented fallback — and a spec now builds identically
// with types.Info reduced to Defs alone.
//
// So the hybrid is merely unbuilt here, and it stays that way because this strategy already has the source in hand:
// reading an annotated dependency in full costs one type-check of a package small enough not to matter, and a fully
// checked package is the simpler thing to hand on. Where it would earn its keep is the go/packages strategy, which
// has no per-dependency lever at all — a LoadMode is one value for the whole load.
func (ld *loadState) carriesAnnotations(importPath string) bool {
	if known, ok := ld.annotated[importPath]; ok {
		return known
	}

	found := ld.scanForAnnotations(importPath)
	ld.annotated[importPath] = found

	return found
}

// annotationMarker is the prefix every codescan annotation begins with.
const annotationMarker = "swagger:"

// annotationChunk is how much of a file the marker scan reads at a time.
//
// NOTE: measured over the standard library, this repository's dependencies and a generated client:
// the median Go source file is under 6 KB and 85–92% of them are under 16 KB, so one read usually covers a whole file.
// The tail is why there is a bound at all — the standard library carries a single 2.9 MB generated file,
// and holding that in memory to look for eight bytes is the cost this avoids.
const annotationChunk = 16 << 10

// markerCarryOver is how much of one chunk the next one starts with.
//
// A marker split across a read boundary is invisible to both halves, and missing one is not a degraded read: the
// package would be served from export data and everything its source said about its own types would go unread, leaving
// a spec that is valid and quieter. So each read keeps the tail a marker could still be starting in.
const markerCarryOver = len(annotationMarker) - 1

// scanForAnnotations looks for the marker in a package's source, without parsing it.
//
// This scan should capture _at least_ what we need as it is an optimization. It doesn't have to resolve the
// exact regular expression in comments, just to discard all obvious unmatched content.
func (ld *loadState) scanForAnnotations(importPath string) bool {
	// One buffer for every file in the package.
	// The scan holds nothing else, so a 2.9 MB source file costs what a 2 KB one does.
	var buf [annotationChunk]byte

	for _, path := range ld.sourceFiles(importPath) {
		if fileCarriesMarker(ld.vfs.Open, path, buf[:]) {
			return true
		}
	}

	return false
}

// sourceFiles locates a dependency's Go source, or says why there is none to locate.
//
// Memoized per import path: the marker scan asks for this during the load and a read-back may ask for it again
// afterwards, and neither wants to resolve the import or stat the directory twice.
//
// The two refusals are the ones a scan can still feel later. A package whose source is not there keeps its types and
// loses whatever its declarations said, which shows up in the output as nothing at all — so it is recorded here and
// replayed at the lookup that wanted a declaration out of it.
func (ld *loadState) sourceFiles(importPath string) []string {
	if known, ok := ld.srcFiles[importPath]; ok {
		return known
	}

	files := ld.resolveSourceFiles(importPath)
	ld.srcFiles[importPath] = files

	return files
}

func (ld *loadState) resolveSourceFiles(importPath string) []string {
	dir, _, ok := ld.res.ResolveImport(importPath)
	if !ok {
		ld.reportExportOnly(importPath, "its source is not on the filesystem")

		return nil
	}

	bp, err := ld.ctx.ImportDir(dir, 0)
	if err != nil {
		ld.reportExportOnly(importPath, "its source could not be read")

		return nil
	}

	files := make([]string, 0, len(bp.GoFiles)+len(bp.CgoFiles))
	for _, name := range bp.GoFiles {
		files = append(files, ld.vfs.Join(dir, name))
	}
	for _, name := range bp.CgoFiles {
		files = append(files, ld.vfs.Join(dir, name))
	}

	return files
}

// fileCarriesMarker reports whether the file at path contains the annotation marker, reading it a chunk at a time and
// stopping at the first hit.
//
// buf belongs to the caller and is reused across a package's files, so the scan allocates once per package rather than
// once per file, and never in proportion to what it is reading. It does not escape analysis — Read is an interface
// method, so the buffer is heap-allocated — which is why it is hoisted to the caller instead of declared here.
//
// open is the caller's, because the two strategies read through different filesystems: the toolchain-free one honours
// a virtual tree, while go/packages hands over paths that only ever exist on the real one.
//
// A file that cannot be read is skipped: the package keeps whatever its other files say.
func fileCarriesMarker(open func(string) (io.ReadCloser, error), path string, buf []byte) bool {
	f, err := open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	marker := []byte(annotationMarker)
	carried := 0

	for {
		n, readErr := f.Read(buf[carried:])
		if n > 0 {
			window := buf[:carried+n]
			if bytes.Contains(window, marker) {
				return true
			}

			// Keep only the tail a marker could still be starting in. copy handles the overlap, since source and
			// destination are the same buffer.
			carried = copy(buf, window[max(0, len(window)-markerCarryOver):])
		}
		if readErr != nil {
			return false
		}
	}
}

// reportExportOnly announces a dependency whose types were read but whose source was not.
func (ld *loadState) reportExportOnly(importPath, why string) {
	if ld.onExportOnly == nil || ld.exportOnlyReported[importPath] {
		return
	}
	ld.exportOnlyReported[importPath] = true

	ld.onExportOnly(ExportOnly{Path: importPath, Reason: why})
}

// importExported returns a package read from export data, completing anything it refers to.
//
// This is the fast path over parsing the file from source, whenever source is not needed immediately by
// codescan to determine the root files that need source parsing.
func (ld *loadState) importExported(importPath string) (*types.Package, error) {
	if pkg, ok := ld.exported[importPath]; ok && pkg.Complete() {
		return pkg, nil
	}
	if ld.exportInProgress[importPath] {
		// Export data has no import cycles, but a corrupt or hand-made tree could; refuse rather than recurse forever.
		return nil, fmt.Errorf("%w through %q in export data", ErrImportCycle, importPath)
	}
	ld.exportInProgress[importPath] = true
	defer delete(ld.exportInProgress, importPath)

	pkg, err := ld.readExported(importPath)
	if err != nil {
		return nil, err
	}

	// Read leaves referenced packages as incomplete placeholders.
	// That is fine until the checker looks inside one — a field whose type lives in another package, say — so complete
	// them eagerly.
	// The closure is only what this package actually refers to, and reading export data is cheap.
	for _, dep := range pkg.Imports() {
		if dep.Complete() || ld.exportInProgress[dep.Path()] {
			continue
		}
		if _, err := ld.importExported(dep.Path()); err != nil {
			// A missing dependency degrades that one package rather than failing the import: the referring package is usually
			// still usable for what codescan asks of it.
			continue
		}
	}

	return pkg, nil
}

func (ld *loadState) readExported(importPath string) (*types.Package, error) {
	blob, err := fs.ReadFile(ld.exportFS, path.Join(importPath)+".export")
	if err != nil {
		return nil, fmt.Errorf("no export data for %q: %w", importPath, err)
	}

	// A generated tree holds bare export sections, which gcexportdata.Read takes directly.
	//
	// A whole compiled archive is accepted too, but has to have its section located first — NewReader only understands
	// the archive form and rejects the bare one.
	var in io.Reader = bytes.NewReader(blob)
	if bytes.HasPrefix(blob, []byte("!<arch>")) {
		if in, err = gcexportdata.NewReader(in); err != nil {
			return nil, fmt.Errorf("locating export data for %q: %w", importPath, err)
		}
	}

	pkg, err := gcexportdata.Read(in, ld.fset, ld.exported, importPath)
	if err != nil {
		return nil, fmt.Errorf("decoding export data for %q: %w", importPath, err)
	}

	return pkg, nil
}

// hasExportData reports whether the configured tree can serve this import path.
func (ld *loadState) hasExportData(importPath string) bool {
	if ld.exportFS == nil {
		return false
	}
	if pkg, ok := ld.exported[importPath]; ok && pkg.Complete() {
		return true
	}

	f, err := ld.exportFS.Open(path.Join(importPath) + ".export")
	if err != nil {
		return false
	}
	_ = f.Close()

	return true
}
