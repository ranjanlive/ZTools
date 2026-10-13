// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var version = "1.0.0"

// Include dotfiles, wrapper scripts, and the binary JAR in the executable.
//
//go:embed all:templates/empty_activity profiles/stable.json android_project_scaffold.json LICENSE NOTICE
var assets embed.FS

type Profile struct {
	ID              string `json:"id"`
	Revision        string `json:"revision"`
	AGP             string `json:"agp"`
	Gradle          string `json:"gradle"`
	GradleSHA       string `json:"gradle_sha256"`
	WrapperSHA      string `json:"wrapper_sha256"`
	Kotlin          string `json:"kotlin"`
	ComposeBOM      string `json:"compose_bom"`
	CoreKTX         string `json:"core_ktx"`
	ActivityCompose string `json:"activity_compose"`
	Lifecycle       string `json:"lifecycle"`
	Splashscreen    string `json:"splashscreen"`
	CompileSDK      int    `json:"compile_sdk"`
	TargetSDK       int    `json:"target_sdk"`
	DefaultMinSDK   int    `json:"default_min_sdk"`
	MinimumMinSDK   int    `json:"minimum_min_sdk"`
	JavaMinimum     int    `json:"java_minimum"`
}

type Request struct {
	Name     string `json:"name"`
	Package  string `json:"package"`
	Output   string `json:"output"`
	MinSDK   *int   `json:"min_sdk,omitempty"`
	Template string `json:"template,omitempty"`
	SDKPath  string `json:"sdk_path,omitempty"`
	DryRun   bool   `json:"dry_run,omitempty"`
}

type Report struct {
	Version     string   `json:"tool_version"`
	ProjectName string   `json:"project_name"`
	GradleName  string   `json:"gradle_project_name"`
	Package     string   `json:"package"`
	ProjectDir  string   `json:"project_dir"`
	Template    string   `json:"template"`
	Profile     Profile  `json:"profile"`
	MinSDK      int      `json:"min_sdk"`
	DryRun      bool     `json:"dry_run"`
	Files       []string `json:"files"`
	Next        []string `json:"next_steps"`
}

type Result struct {
	Output  string  `json:"output"`
	IsError bool    `json:"is_error"`
	Report  *Report `json:"report,omitempty"`
}

type generatedFile struct {
	Name string
	Data []byte
	Mode fs.FileMode
}

var packagePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
var tokenPattern = regexp.MustCompile(`__[A-Z][A-Z0-9_]*__`)
var keywords = strings.Fields("abstract actual annotation as assert boolean break byte by case catch char class companion const constructor continue crossinline data default delegate do double dynamic else enum expect external false field final finally float for fun get goto if implements import in infix init inline instanceof int interface internal is it lateinit long native new noinline null object open operator out override package param private property protected public receiver reified return sealed set short static strictfp super suspend synchronized tailrec this throw throws transient true try typealias typeof val var vararg void volatile when where while yield")

func loadProfile() (Profile, error) {
	var p Profile
	b, err := assets.ReadFile("profiles/stable.json")
	if err == nil {
		err = json.Unmarshal(b, &p)
	}
	return p, err
}

func validate(r Request, p Profile) (Request, error) {
	if !utf8.ValidString(r.Name) || strings.TrimSpace(r.Name) == "" || utf8.RuneCountInString(r.Name) > 80 {
		return r, errors.New("name must contain 1–80 Unicode characters and cannot be blank")
	}
	for _, c := range r.Name {
		if unicode.IsControl(c) {
			return r, errors.New("name cannot contain control characters")
		}
	}
	if len(r.Package) > 150 || !packagePattern.MatchString(r.Package) {
		return r, errors.New("package must contain at least two lowercase ASCII identifiers, e.g. com.example.app")
	}
	for _, segment := range strings.Split(r.Package, ".") {
		for _, keyword := range keywords {
			if segment == keyword {
				return r, fmt.Errorf("package segment %q is a reserved identifier", segment)
			}
		}
	}
	if r.Output == "" {
		return r, errors.New("output must name a new destination directory")
	}
	if r.Template == "" {
		r.Template = "empty_activity"
	}
	if r.Template != "empty_activity" {
		return r, errors.New("template must be empty_activity")
	}
	if r.MinSDK == nil {
		sdk := p.DefaultMinSDK
		r.MinSDK = &sdk
	}
	if *r.MinSDK < p.MinimumMinSDK || *r.MinSDK > p.TargetSDK {
		return r, fmt.Errorf("min_sdk must be between %d and %d for this profile", p.MinimumMinSDK, p.TargetSDK)
	}
	target, err := canonicalTarget(r.Output)
	if err != nil {
		return r, fmt.Errorf("output: %w", err)
	}
	r.Output = target
	if env := os.Getenv("ZAGENT_WORKSPACE"); env != "" {
		workspace, err := filepath.Abs(env)
		if err == nil {
			workspace, err = filepath.EvalSymlinks(workspace)
		}
		if err != nil {
			return r, fmt.Errorf("cannot resolve ZAGENT_WORKSPACE: %w", err)
		}
		relative, err := filepath.Rel(workspace, target)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return r, errors.New("agent output must be a new directory inside ZAGENT_WORKSPACE")
		}
	}
	if _, err := os.Lstat(target); err == nil {
		return r, errors.New("destination already exists; choose a new directory (existing projects are never overwritten)")
	} else if !os.IsNotExist(err) {
		return r, err
	}
	if r.SDKPath != "" {
		r.SDKPath, err = filepath.Abs(r.SDKPath)
		if err != nil {
			return r, err
		}
		info, err := os.Stat(r.SDKPath)
		if err != nil || !info.IsDir() {
			return r, errors.New("sdk_path must refer to an existing Android SDK directory")
		}
	}
	return r, nil
}

// Resolve existing ancestors before containment checks. Nonexistent descendants
// remain under that resolved ancestor rather than following textual symlink paths.
func canonicalTarget(input string) (string, error) {
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	if abs == filepath.VolumeName(abs)+string(filepath.Separator) {
		return "", errors.New("filesystem root cannot be a destination")
	}
	ancestor := filepath.Dir(abs)
	suffix := []string{filepath.Base(abs)}
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		suffix = append(suffix, filepath.Base(ancestor))
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("cannot resolve destination parent")
		}
		ancestor = parent
	}
	ancestor, err = filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(ancestor)
	if err != nil || !info.IsDir() {
		return "", errors.New("destination parent is not a directory")
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		ancestor = filepath.Join(ancestor, suffix[i])
	}
	return ancestor, nil
}

func kotlinString(s string) string {
	// Quoting the entire literal also escapes Gradle/Kotlin interpolation.
	return strings.ReplaceAll(strconv.Quote(s), "$", `\$`)
}

func androidString(s string) string {
	// Android resource escaping is separate from XML escaping; outer quotes
	// preserve whitespace and make apostrophes literal. formatted=false handles %.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(`"`+s+`"`))
	return b.String()
}

func propertiesString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', ':', '=', '#', '!', ' ':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r < 32 || r > 126 {
				if r <= 0xffff {
					fmt.Fprintf(&b, `\u%04x`, r)
				} else {
					n := r - 0x10000
					fmt.Fprintf(&b, `\u%04x\u%04x`, 0xd800+(n>>10), 0xdc00+(n&1023))
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func renderFiles(r Request, p Profile) ([]generatedFile, error) {
	values := map[string]string{
		"__PROJECT_NAME__": strings.Split(r.Package, ".")[len(strings.Split(r.Package, "."))-1],
		"__APP_NAME_XML__": androidString(r.Name), "__PACKAGE_NAME__": r.Package,
		"__PACKAGE_PATH__": strings.ReplaceAll(r.Package, ".", "/"), "__THEME_NAME__": "App",
		"__MIN_SDK__": strconv.Itoa(*r.MinSDK), "__COMPILE_SDK__": strconv.Itoa(p.CompileSDK), "__TARGET_SDK__": strconv.Itoa(p.TargetSDK),
		"__AGP__": p.AGP, "__KOTLIN__": p.Kotlin, "__COMPOSE_BOM__": p.ComposeBOM, "__CORE_KTX__": p.CoreKTX,
		"__ACTIVITY_COMPOSE__": p.ActivityCompose, "__LIFECYCLE__": p.Lifecycle, "__SPLASHSCREEN__": p.Splashscreen,
	}
	pairs := make([]string, 0, len(values)*2)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pairs = append(pairs, key, values[key])
	}
	replacer := strings.NewReplacer(pairs...)
	var files []generatedFile
	prefix := "templates/" + r.Template + "/"
	err := fs.WalkDir(assets, strings.TrimSuffix(prefix, "/"), func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := assets.ReadFile(name)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(name, prefix)
		mode := fs.FileMode(0644)
		if relative == "gradle/wrapper/gradle-wrapper.jar" {
			if fmt.Sprintf("%x", sha256.Sum256(data)) != p.WrapperSHA {
				return errors.New("embedded Gradle wrapper checksum mismatch")
			}
		} else {
			for _, token := range tokenPattern.FindAllString(string(data), -1) {
				if _, ok := values[token]; !ok {
					return fmt.Errorf("unrecognized template token %s in %s", token, relative)
				}
			}
			data = []byte(replacer.Replace(string(data)))
		}
		relative = replacer.Replace(relative)
		if relative == "gradlew" {
			mode = 0755
		}
		files = append(files, generatedFile{relative, data, mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if r.SDKPath != "" {
		files = append(files, generatedFile{"local.properties", []byte("# Machine-local Android SDK; do not commit.\nsdk.dir=" + propertiesString(r.SDKPath) + "\n"), 0644})
	}
	return files, nil
}

// Reserve the destination exclusively so simultaneous agents cannot overwrite
// each other. Rollback removes only paths created by this invocation, never
// recursively deletes a user's destination or preexisting parent directory.
func writeNewDirectory(destination string, files []generatedFile) (err error) {
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(destination, 0755); err != nil {
		return fmt.Errorf("cannot reserve destination (it may already exist): %w", err)
	}
	directories := []string{destination}
	written := []string{}
	defer func() {
		if err != nil {
			for i := len(written) - 1; i >= 0; i-- {
				_ = os.Remove(written[i])
			}
			for i := len(directories) - 1; i >= 0; i-- {
				_ = os.Remove(directories[i])
			}
		}
	}()
	for _, file := range files {
		rel := filepath.FromSlash(file.Name)
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("invalid internal template path")
		}
		current := destination
		components := strings.Split(filepath.Dir(rel), string(filepath.Separator))
		for _, component := range components {
			if component == "." {
				continue
			}
			current = filepath.Join(current, component)
			e := os.Mkdir(current, 0755)
			if e == nil {
				directories = append(directories, current)
			} else if !os.IsExist(e) {
				return e
			} else {
				info, e := os.Lstat(current)
				if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return errors.New("generated directory was replaced during generation")
				}
			}
		}
		target := filepath.Join(destination, rel)
		f, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.Mode)
		if e != nil {
			return e
		}
		written = append(written, target)
		_, e = f.Write(file.Data)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func generate(r Request) (Report, error) {
	p, err := loadProfile()
	if err != nil {
		return Report{}, err
	}
	r, err = validate(r, p)
	if err != nil {
		return Report{}, err
	}
	files, err := renderFiles(r, p)
	if err != nil {
		return Report{}, err
	}
	report := Report{Version: version, ProjectName: r.Name, GradleName: strings.Split(r.Package, ".")[len(strings.Split(r.Package, "."))-1], Package: r.Package, ProjectDir: r.Output, Template: r.Template, Profile: p, MinSDK: *r.MinSDK, DryRun: r.DryRun}
	report.Next = []string{"Open project_dir in an Android Studio version supporting AGP " + p.AGP + ".", "Configure Java 17+ and Android SDK platform 37 (unless sdk_path was supplied, set ANDROID_HOME or let Android Studio create local.properties).", "From project_dir run gradlew.bat assembleDebug lintDebug testDebugUnitTest on Windows, or ./gradlew assembleDebug lintDebug testDebugUnitTest on Linux/macOS."}
	for _, file := range files {
		report.Files = append(report.Files, file.Name)
	}
	report.Files = append(report.Files, ".scaffold.json")
	sort.Strings(report.Files)
	metadata, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Report{}, err
	}
	files = append(files, generatedFile{".scaffold.json", append(metadata, '\n'), 0644})
	if !r.DryRun {
		err = writeNewDirectory(r.Output, files)
	}
	return report, err
}

func manifest(binaryName string) ([]byte, error) {
	data, err := assets.ReadFile("android_project_scaffold.json")
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	m["command"] = []string{"{dir}/" + binaryName, "--json"}
	data, err = json.MarshalIndent(m, "", "  ")
	return append(data, '\n'), err
}

func install(home string) (string, error) {
	if home == "" {
		return "", errors.New("install requires an agent home directory")
	}
	binaryName := "android_project_scaffold"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		return "", err
	}
	m, err := manifest(binaryName)
	if err != nil {
		return "", err
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(home, "tools", "android_project_scaffold")
	license, err := assets.ReadFile("LICENSE")
	if err != nil {
		return "", err
	}
	notice, err := assets.ReadFile("NOTICE")
	if err != nil {
		return "", err
	}
	err = writeNewDirectory(dest, []generatedFile{{binaryName, binary, 0755}, {"android_project_scaffold.json", m, 0644}, {"LICENSE", license, 0644}, {"NOTICE", notice, 0644}})
	return dest, err
}

func decodeRequest(input io.Reader) (Request, error) {
	var r Request
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil {
		return r, err
	}
	if len(data) > 65536 {
		return r, errors.New("JSON input exceeds 64 KiB")
	}
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return r, errors.New("stdin must contain one JSON arguments object")
	}
	// encoding/json accepts null for primitive fields. Reject explicit nulls so
	// defaults cannot silently turn malformed agent arguments into another request.
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(trimmed, &raw); err != nil {
		return r, err
	}
	for key, value := range raw {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return r, fmt.Errorf("%s cannot be null", key)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&r); err != nil {
		return r, err
	}
	return r, nil
}

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	flags := flag.NewFlagSet("android_project_scaffold", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	jsonMode := flags.Bool("json", false, "Read one JSON request from stdin; emit one JSON result")
	showVersion := flags.Bool("version", false, "Print the release version")
	showProfile := flags.Bool("profile", false, "Print the embedded toolchain profile")
	installHome := flags.String("install", "", "Install this binary and its manifest into an agent home")
	var request Request
	flags.StringVar(&request.Name, "name", "", "App display/project name (required)")
	flags.StringVar(&request.Package, "package", "", "Application ID, e.g. com.example.app (required)")
	flags.StringVar(&request.Output, "output", "", "New destination directory (required)")
	flags.StringVar(&request.Template, "template", "empty_activity", "Template name")
	flags.StringVar(&request.SDKPath, "sdk-path", "", "Optional existing Android SDK directory")
	flags.BoolVar(&request.DryRun, "dry-run", false, "Validate and list files without writing")
	minSDK := flags.Int("min-sdk", 26, "Minimum Android API level (26–37)")
	flags.Usage = func() {
		fmt.Fprintln(diagnostics, "Usage: android_project_scaffold --name MyApp --package com.example.myapp --output ./MyApp\n       android_project_scaffold --json < request.json\n       android_project_scaffold --install <agent-home>\nGeneration is offline; existing destinations are always refused.")
		flags.PrintDefaults()
	}
	emitError := func(err error) int {
		_ = json.NewEncoder(output).Encode(Result{Output: err.Error(), IsError: true})
		return 1
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return emitError(err)
	}
	if flags.NArg() != 0 {
		return emitError(errors.New("unexpected positional arguments"))
	}
	visited := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	special := 0
	for _, key := range []string{"json", "version", "profile", "install"} {
		if visited[key] {
			special++
		}
	}
	if special > 0 && len(visited) > 1 {
		return emitError(errors.New("--json, --version, --profile and --install must be used alone"))
	}
	if *showVersion {
		fmt.Fprintln(output, version)
		return 0
	}
	if *showProfile {
		p, err := loadProfile()
		if err != nil {
			return emitError(err)
		}
		_ = json.NewEncoder(output).Encode(p)
		return 0
	}
	if visited["install"] {
		dest, err := install(*installHome)
		if err != nil {
			return emitError(err)
		}
		_ = json.NewEncoder(output).Encode(Result{Output: "Installed in " + dest + ". Start a new agent session."})
		return 0
	}
	var err error
	if *jsonMode {
		request, err = decodeRequest(input)
	} else {
		request.MinSDK = minSDK
	}
	if err != nil {
		return emitError(err)
	}
	report, err := generate(request)
	if err != nil {
		return emitError(err)
	}
	summary, err := json.Marshal(report)
	if err != nil {
		return emitError(err)
	}
	// The engine consumes output as text; include the structured report there as
	// well as under report for standalone automation clients.
	_ = json.NewEncoder(output).Encode(Result{Output: string(summary), Report: &report})
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
