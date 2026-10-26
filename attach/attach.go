// Package attach works out how to build a program written for the headset so it runs on the
// computer, in FVR's window, without changing a file of it: which of its files to show the
// compiler as they would be on the headset, which to hide, and the one to add that brings FVR in.
// The go command takes the answer as an overlay (go build -overlay), a list of files it reads in
// place of the ones on disk.
//
// A headset program keeps its start on the headset behind a build constraint (//go:build android,
// or a file named _android.go) and often has a file for elsewhere that only says to build it for
// the headset. Here the first is shown without its constraint and the second hidden; everything
// else is left as it is.
package attach

import (
	"go/build/constraint"
	"path/filepath"
	"strings"
)

// FVRImport is the package that makes app.Run run in FVR's window when a program imports it.
const FVRImport = "github.com/akrck02/FVR/headset"

// AddedFile is the name of the file added to the program's package, importing FVRImport.
const AddedFile = "fvr_attach.go"

// Platform is where a file is built: the tags its constraints are judged by.
type Platform struct {
	OS, Architecture string
	// Tags are the build's own tags (go build -tags), on top of the system's.
	Tags []string
}

// Headset is a Meta Quest: Android on 64-bit ARM.
var Headset = Platform{OS: "android", Architecture: "arm64"}

// unixes are the systems the "unix" constraint names, of those a program may be built on here.
var unixes = map[string]bool{"android": true, "darwin": true, "dragonfly": true, "freebsd": true, "illumos": true,
	"ios": true, "linux": true, "netbsd": true, "openbsd": true, "solaris": true, "aix": true, "hurd": true}

// knownOperatingSystems and knownArchitectures are the ones a file name's suffix can name (as
// go/build reads them).
var (
	knownOperatingSystems = strings.Fields("aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos")
	knownArchitectures    = strings.Fields("386 amd64 arm arm64 loong64 mips mips64 mips64le mipsle ppc64 ppc64le riscv64 s390x wasm")
)

// has tells whether a build tag holds on the platform. Cgo is taken as on (both the headset and
// FVR's window need it), and so is any go1.N version.
func (platform Platform) has(tag string) bool {
	switch {
	case tag == platform.OS || tag == platform.Architecture:
		return true
	case tag == "linux" && platform.OS == "android", tag == "darwin" && platform.OS == "ios":
		return true
	case tag == "unix":
		return unixes[platform.OS]
	case tag == "cgo", tag == "gc", strings.HasPrefix(tag, "go1."):
		return true
	}
	return known(tag, platform.Tags)
}

// Builds tells whether a Go file of that name and source is built on the platform: its name's
// _GOOS / _GOARCH suffix and its //go:build line both allow it.
func (platform Platform) Builds(name, source string) bool {
	if !platform.nameAllows(name) {
		return false
	}
	expression, found := buildConstraint(source)
	return !found || expression.Eval(platform.has)
}

// nameAllows reads a file name's suffix as go/build does: name_GOOS.go, name_GOARCH.go or
// name_GOOS_GOARCH.go are built only there.
func (platform Platform) nameAllows(name string) bool {
	parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(filepath.Base(name), ".go"), "_test"), "_")
	if len(parts) < 2 {
		return true
	}
	last := parts[len(parts)-1]
	if len(parts) >= 3 && known(parts[len(parts)-2], knownOperatingSystems) && known(last, knownArchitectures) {
		return platform.has(parts[len(parts)-2]) && platform.has(last)
	}
	if known(last, knownOperatingSystems) || known(last, knownArchitectures) {
		return platform.has(last)
	}
	return true
}

func known(word string, words []string) bool {
	for _, each := range words {
		if each == word {
			return true
		}
	}
	return false
}

// buildConstraint is a source's //go:build line, if it has one before its package clause.
func buildConstraint(source string) (constraint.Expr, bool) {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
		if constraint.IsGoBuild(trimmed) {
			if expression, err := constraint.Parse(trimmed); err == nil {
				return expression, true
			}
		}
	}
	return nil, false
}

// withoutConstraint is a source with its //go:build and // +build lines taken out.
func withoutConstraint(source string) string {
	lines := strings.Split(source, "\n")
	kept := lines[:0]
	inHeader := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			inHeader = false
		}
		if inHeader && (constraint.IsGoBuild(trimmed) || constraint.IsPlusBuild(trimmed)) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// Change is what the compiler is shown in place of a file of the program's package.
type Change struct {
	// Path is the file, in the package's folder: an existing one, or a new one (Hidden false,
	// Source the whole file).
	Path string
	// Hidden hides the file from the compiler.
	Hidden bool
	// Source is what the compiler reads at Path, when it is not hidden.
	Source string
}

// Changes are the changes that build a package of the headset (its folder dir, its files' names
// and sources, its name) on the platform, the headset built with the platform's tags too: the files built only on the headset shown without what
// keeps them there (under a new name when it is their name), the files built only here hidden,
// and AddedFile importing FVRImport. Tests are left alone. It returns nothing when no file is built
// only on the headset: that package needs no help to run here.
func Changes(dir string, sources map[string]string, packageName string, platform Platform) []Change {
	headset := Platform{OS: Headset.OS, Architecture: Headset.Architecture, Tags: platform.Tags}
	var changes []Change
	for name, source := range sources {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		onHeadset, here := headset.Builds(name, source), platform.Builds(name, source)
		switch {
		case onHeadset && !here:
			shown := name
			if !platform.nameAllows(name) {
				shown = strings.TrimSuffix(name, ".go") + "_fvr.go"
			}
			changes = append(changes, Change{Path: filepath.Join(dir, shown), Source: withoutConstraint(source)})
		case here && !onHeadset:
			changes = append(changes, Change{Path: filepath.Join(dir, name), Hidden: true})
		}
	}
	if !anyShown(changes) {
		return nil
	}
	added := "// Added by fvr: the program runs in FVR's window.\n\npackage " + packageName + "\n\nimport _ \"" + FVRImport + "\"\n"
	return append(changes, Change{Path: filepath.Join(dir, AddedFile), Source: added})
}

func anyShown(changes []Change) bool {
	for _, change := range changes {
		if !change.Hidden {
			return true
		}
	}
	return false
}
