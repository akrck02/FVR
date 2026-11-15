package attach

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var mac = Platform{OS: "darwin", Architecture: "arm64"}

func TestTheHeadsetAndTheComputerBuildWhatTheirConstraintsSay(test *testing.T) {
	for _, each := range []struct {
		name, source          string
		onHeadset, onComputer bool
	}{
		{"main.go", "//go:build android\n\npackage main\n", true, false},
		{"elsewhere.go", "//go:build !android\n\npackage main\n", false, true},
		{"world.go", "package main\n", true, true},
		{"headset_android.go", "package main\n", true, false},
		{"desktop_darwin.go", "package main\n", false, true},
		{"loop_linux.go", "package main\n", true, false},
		{"either.go", "//go:build android || darwin\n\npackage main\n", true, true},
		{"cgo.go", "//go:build cgo && unix\n\npackage main\n", true, true},
	} {
		if got := Headset.Builds(each.name, each.source); got != each.onHeadset {
			test.Errorf("%s on the headset: %v", each.name, got)
		}
		if got := mac.Builds(each.name, each.source); got != each.onComputer {
			test.Errorf("%s on the computer: %v", each.name, got)
		}
	}
}

func TestAHeadsetProgramIsShownAsOnTheHeadsetAndFVRAdded(test *testing.T) {
	sources := map[string]string{
		"main.go":      "//go:build android\n\n// The program.\npackage main\n\nfunc main() {}\n",
		"elsewhere.go": "//go:build !android\n\npackage main\n\nfunc main() {}\n",
		"board.go":     "package main\n",
		"main_test.go": "//go:build android\n\npackage main\n",
	}
	changes := Changes("/game", sources, "main", mac)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	if len(changes) != 3 {
		test.Fatalf("changes %+v", changes)
	}
	elsewhere, added, main := changes[0], changes[1], changes[2]
	if elsewhere.Path != filepath.Join("/game", "elsewhere.go") || !elsewhere.Hidden {
		test.Fatalf("elsewhere.go: %+v", elsewhere)
	}
	if main.Hidden || strings.Contains(main.Source, "go:build") || !strings.Contains(main.Source, "// The program.\npackage main") {
		test.Fatalf("main.go shown as %q", main.Source)
	}
	if added.Path != filepath.Join("/game", AddedFile) || !strings.Contains(added.Source, `import _ "`+FVRImport+`"`) {
		test.Fatalf("added %+v", added)
	}
}

func TestAFileKeptOnTheHeadsetByItsNameIsShownUnderAnother(test *testing.T) {
	changes := Changes("/game", map[string]string{"headset_android.go": "package main\n"}, "main", mac)
	var paths []string
	for _, change := range changes {
		paths = append(paths, filepath.Base(change.Path))
	}
	sort.Strings(paths)
	if strings.Join(paths, " ") != AddedFile+" headset_android_fvr.go" {
		test.Fatalf("paths %v", paths)
	}
	if !mac.Builds("headset_android_fvr.go", "package main\n") {
		test.Fatal("the new name is still kept off the computer")
	}
}

func TestAPackageThatAlreadyBuildsHereNeedsNothing(test *testing.T) {
	if changes := Changes("/game", map[string]string{"main.go": "package main\n"}, "main", mac); changes != nil {
		test.Fatalf("changes %+v", changes)
	}
}

func TestTheBuildsTagsCountOnTheHeadsetAndHere(test *testing.T) {
	sources := map[string]string{
		"headset_android.go": "//go:build demo && quest && android\n\npackage main\n",
		"headset_other.go":   "//go:build demo && quest && !android\n\npackage main\n",
		"main_screen.go":     "//go:build !quest\n\npackage main\n",
	}
	if changes := Changes("/game", sources, "main", mac); changes != nil {
		test.Fatalf("without the tags nothing is the headset's, got %+v", changes)
	}
	tagged := mac
	tagged.Tags = []string{"demo", "quest"}
	var paths []string
	for _, change := range Changes("/game", sources, "main", tagged) {
		state := "shown"
		if change.Hidden {
			state = "hidden"
		}
		paths = append(paths, filepath.Base(change.Path)+" "+state)
	}
	sort.Strings(paths)
	if got := strings.Join(paths, ", "); got != AddedFile+" shown, headset_android_fvr.go shown, headset_other.go hidden" {
		test.Fatalf("changes %s", got)
	}
}
