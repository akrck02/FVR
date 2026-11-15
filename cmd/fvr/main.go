// fvr runs a program written for the headset (go-meta-quest's app.Run or explore.Run) on the
// computer, in FVR's window, without changing a file of it:
//
//	go run ./cmd/fvr ../go-meta-quest/cmd/explorer
//	go run ./cmd/fvr -both ../go-meta-quest/cmd/panel -- -some-flag-of-the-program
//	go run ./cmd/fvr -capture explorer.png -frame 60 ../go-meta-quest/cmd/explorer
//	go run ./cmd/fvr -tags demo,quest ../a-game
//
// -both shows both eyes, and -capture saves a frame (-frame, the first by default) as a PNG and ends
// the program; they reach FVR's window through its environment (headset.WindowFromEnvironment).
// -tags are the program's build tags, as go build takes them: the ones its headset build uses.
//
// It builds the program with go build in a workspace of its own (a go.work in a temporary folder,
// with FVR, the modules FVR uses and the program's module), showing the compiler the program's
// files as they are on the headset and adding one that imports FVR's headset package
// (attach.Changes, through go build -overlay), and runs it in that temporary folder, as far from its
// source as on the headset: what it writes where it runs stays out of the program's repository. What
// follows -- is handed to the program.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/akrck02/FVR/attach"
)

// fvrModules are the modules FVR builds with, put in the workspace from where FVR has them.
var fvrModules = []string{"github.com/akrck02/go-meta-quest", "github.com/akrck02/go-renderer"}

func main() {
	both := flag.Bool("both", false, "show both eyes side by side")
	capture := flag.String("capture", "", "save the window as a PNG at this path, then end")
	frame := flag.Int("frame", 0, "the frame -capture saves (the first when 0)")
	tags := flag.String("tags", "", "the program's build tags, comma-separated, as go build -tags")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "uso: go run ./cmd/fvr [-both] [-tags a,b] [-capture fichero.png [-frame n]] <carpeta del programa> [-- argumentos del programa]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *both {
		os.Setenv("FVR_BOTH", "1")
	}
	if *capture != "" {
		path, err := filepath.Abs(*capture)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fvr:", err)
			os.Exit(2)
		}
		os.Setenv("FVR_CAPTURE", path)
		os.Setenv("FVR_FRAME", strconv.Itoa(*frame))
	}
	code, err := run(flag.Arg(0), buildTags(*tags), programArguments(flag.Args()[1:]))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fvr:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// buildTags are the tags of a -tags flag: comma-separated, or space-separated as go build once took them.
func buildTags(flag string) []string {
	return strings.FieldsFunc(flag, func(character rune) bool { return character == ',' || character == ' ' })
}

// programArguments are the arguments after --, or all of them when there is none.
func programArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[index+1:]
		}
	}
	return arguments
}

func run(program string, tags, arguments []string) (int, error) {
	dir, err := filepath.Abs(program)
	if err != nil {
		return 0, err
	}
	packageName, sources, err := readPackage(dir)
	if err != nil {
		return 0, err
	}
	changes := attach.Changes(dir, sources, packageName, attach.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH, Tags: tags})
	if changes == nil {
		return 0, fmt.Errorf("%s no tiene nada solo para el casco: ejecútalo con go run directamente", program)
	}
	temporary, err := os.MkdirTemp("", "fvr-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(temporary)
	workspace, err := writeWorkspace(temporary, dir)
	if err != nil {
		return 0, err
	}
	overlay, err := writeOverlay(temporary, changes)
	if err != nil {
		return 0, err
	}
	built := filepath.Join(temporary, "program")
	build := exec.Command("go", "build", "-tags="+strings.Join(tags, ","), "-overlay="+overlay, "-o", built, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK="+workspace)
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		return 0, fmt.Errorf("no compila: %w", err)
	}
	command := exec.Command(built, arguments...)
	command.Dir = temporary
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}

// readPackage reads the Go files of a folder, and the name of the package they make.
func readPackage(dir string) (string, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	sources := map[string]string{}
	packageName := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", nil, err
		}
		sources[name] = string(source)
		if packageName == "" && !strings.HasSuffix(name, "_test.go") {
			parsed, err := parser.ParseFile(token.NewFileSet(), name, source, parser.PackageClauseOnly)
			if err == nil {
				packageName = parsed.Name.Name
			}
		}
	}
	if packageName == "" {
		return "", nil, fmt.Errorf("%s no tiene ficheros Go", dir)
	}
	return packageName, sources, nil
}

// writeWorkspace writes a go.work using FVR, the modules FVR builds with and the program's module.
func writeWorkspace(temporary, program string) (string, error) {
	root, err := fvrRoot()
	if err != nil {
		return "", err
	}
	modules := []string{root}
	for _, module := range fvrModules {
		dir, err := output(root, "go", "list", "-m", "-f", "{{.Dir}}", module)
		if err != nil {
			return "", fmt.Errorf("FVR no encuentra %s: %w", module, err)
		}
		modules = append(modules, dir)
	}
	programModule, err := output(program, "go", "env", "GOMOD")
	if err != nil || programModule == "" || programModule == os.DevNull {
		return "", fmt.Errorf("%s no está en un módulo Go", program)
	}
	modules = append(modules, filepath.Dir(programModule))
	version, err := output(root, "go", "env", "GOVERSION")
	if err != nil {
		return "", err
	}
	var work strings.Builder
	fmt.Fprintf(&work, "go %s\n\nuse (\n", strings.TrimPrefix(version, "go"))
	seen := map[string]bool{}
	for _, module := range modules {
		if !seen[module] {
			seen[module] = true
			fmt.Fprintf(&work, "\t%q\n", module)
		}
	}
	work.WriteString(")\n")
	path := filepath.Join(temporary, "go.work")
	return path, os.WriteFile(path, []byte(work.String()), 0o644)
}

// fvrRoot is FVR's own folder: where this command's source is, or the working folder.
func fvrRoot() (string, error) {
	if _, source, _, found := runtime.Caller(0); found {
		root := filepath.Dir(filepath.Dir(filepath.Dir(source)))
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root, nil
		}
	}
	dir, err := output(".", "go", "list", "-m", "-f", "{{.Dir}}", "github.com/akrck02/FVR")
	if err != nil {
		return "", errors.New("no encuentro FVR: ejecuta fvr desde su carpeta")
	}
	return dir, nil
}

// writeOverlay writes the changes as the go command's overlay file, and the files it points at.
func writeOverlay(temporary string, changes []attach.Change) (string, error) {
	replace := map[string]string{}
	for index, change := range changes {
		if change.Hidden {
			replace[change.Path] = ""
			continue
		}
		path := filepath.Join(temporary, fmt.Sprintf("%d_%s", index, filepath.Base(change.Path)))
		if err := os.WriteFile(path, []byte(change.Source), 0o644); err != nil {
			return "", err
		}
		replace[change.Path] = path
	}
	data, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return "", err
	}
	path := filepath.Join(temporary, "overlay.json")
	return path, os.WriteFile(path, data, 0o644)
}

// output runs a command in a folder and returns what it wrote, trimmed.
func output(dir, name string, arguments ...string) (string, error) {
	command := exec.Command(name, arguments...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	written, err := command.Output()
	return strings.TrimSpace(string(written)), err
}
