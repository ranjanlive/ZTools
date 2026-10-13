package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func sampleRequest(t *testing.T) Request {
	t.Helper()
	t.Setenv("ZAGENT_WORKSPACE", "")
	return Request{Name: "My App", Package: "com.example.myapp", Output: filepath.Join(t.TempDir(), "new project")}
}

func TestSpecialNamesAndCompleteProject(t *testing.T) {
	names := []string{`Fish & Chips <"special"> 100% $value`, `123 App`, `O'Brien \ "App"`, "नमस्ते 🚀", "  My App  ", "__PACKAGE_NAME__"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			request := sampleRequest(t)
			request.Name = name
			report, err := generate(request)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Files) < 25 {
				t.Fatalf("incomplete project: %d files", len(report.Files))
			}
			for _, file := range report.Files {
				data, err := os.ReadFile(filepath.Join(report.ProjectDir, filepath.FromSlash(file)))
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(file, ".xml") {
					decoder := xml.NewDecoder(bytes.NewReader(data))
					for {
						_, err := decoder.Token()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatalf("invalid %s: %v", file, err)
						}
					}
				}
			}
			settings, err := os.ReadFile(filepath.Join(report.ProjectDir, "settings.gradle.kts"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(settings), "rootProject.name = "+kotlinString("myapp")) {
				t.Fatalf("name not safely quoted: %s", settings)
			}
			main, err := os.ReadFile(filepath.Join(report.ProjectDir, "app/src/main/java/com/example/myapp/MainActivity.kt"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(main), "AppTheme") || strings.Contains(string(main), "123AppTheme") {
				t.Fatal("display name affected Kotlin identifier")
			}
		})
	}
}

func TestInvalidInputsWriteNothing(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Request)
	}{
		{"blank name", func(r *Request) { r.Name = "  " }},
		{"newline", func(r *Request) { r.Name = "App\nInjected" }},
		{"long name", func(r *Request) { r.Name = strings.Repeat("a", 81) }},
		{"uppercase package", func(r *Request) { r.Package = "com.Example.app" }},
		{"keyword", func(r *Request) { r.Package = "com.class.app" }},
		{"kotlin keyword", func(r *Request) { r.Package = "com.when.app" }},
		{"path package", func(r *Request) { r.Package = "com../outside" }},
		{"missing output", func(r *Request) { r.Output = "" }},
		{"unsupported template", func(r *Request) { r.Template = "../../template" }},
		{"zero SDK", func(r *Request) { v := 0; r.MinSDK = &v }},
		{"low SDK", func(r *Request) { v := 25; r.MinSDK = &v }},
		{"high SDK", func(r *Request) { v := 38; r.MinSDK = &v }},
		{"missing SDK path", func(r *Request) { r.SDKPath = filepath.Join(r.Output, "missing-sdk") }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			r := sampleRequest(t)
			target := r.Output
			test.change(&r)
			if _, err := generate(r); err == nil {
				t.Fatal("accepted invalid request")
			}
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				t.Fatalf("destination modified: %v", err)
			}
		})
	}
}

func TestExistingDestinationAndConcurrentCalls(t *testing.T) {
	r := sampleRequest(t)
	const count = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := generate(r)
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("expected one reservation, got %d", successes)
	}
	marker := filepath.Join(r.Output, "user-code.kt")
	if err := os.WriteFile(marker, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(r); err == nil {
		t.Fatal("overwrote existing destination")
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "existing" {
		t.Fatal("lost existing code")
	}
}

func TestDryRunAndSDKProperties(t *testing.T) {
	r := sampleRequest(t)
	r.DryRun = true
	sdk := filepath.Join(t.TempDir(), "SDK Space नमस्ते")
	if err := os.Mkdir(sdk, 0755); err != nil {
		t.Fatal(err)
	}
	r.SDKPath = sdk
	report, err := generate(r)
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || report.MinSDK != 26 {
		t.Fatal("wrong defaults")
	}
	if _, err := os.Stat(r.Output); !os.IsNotExist(err) {
		t.Fatal("dry run wrote destination")
	}
	r.DryRun = false
	if _, err := generate(r); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(r.Output, "local.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `\ `) || !strings.Contains(string(data), `\u0928`) {
		t.Fatalf("SDK properties not escaped: %s", data)
	}
}

func TestAgentWorkspaceContainment(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("ZAGENT_WORKSPACE", workspace)
	r := Request{Name: "App", Package: "com.example.app", Output: filepath.Join(t.TempDir(), "outside")}
	if _, err := generate(r); err == nil {
		t.Fatal("allowed outside agent workspace")
	}
	r.Output = filepath.Join(workspace, "inside")
	if _, err := generate(r); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Log("symlink test unavailable:", err)
		return
	}
	r.Output = filepath.Join(link, "escaped")
	if _, err := generate(r); err == nil {
		t.Fatal("allowed symlink escape")
	}
}

func TestRollbackDoesNotDeleteUnownedFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "project")
	files := []generatedFile{{"first.txt", []byte("first"), 0644}, {"first.txt", []byte("duplicate"), 0644}}
	if err := writeNewDirectory(destination, files); err == nil {
		t.Fatal("expected exclusive file conflict")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("partial project left behind: %v", err)
	}
	// Invalid internal paths must not write into the parent directory.
	if err := writeNewDirectory(destination, []generatedFile{{"../outside", nil, 0644}}); err == nil {
		t.Fatal("accepted internal traversal")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(destination), "outside")); !os.IsNotExist(err) {
		t.Fatal("escaped destination")
	}
}

func TestJSONProtocolAndCLI(t *testing.T) {
	r := sampleRequest(t)
	r.DryRun = true
	b, _ := json.Marshal(r)
	var output, diagnostics bytes.Buffer
	code := run([]string{"--json"}, bytes.NewReader(b), &output, &diagnostics)
	if code != 0 {
		t.Fatalf("%s %s", &output, &diagnostics)
	}
	var result Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || result.Report == nil || !json.Valid([]byte(result.Output)) {
		t.Fatal("engine result is missing report")
	}
	invalid := []string{`null`, `[]`, `{"name":null}`, `{"name":"a","unknown":true}`, `{} {}`, `{"min_sdk":26.5}`, strings.Repeat("a", 65537)}
	for _, input := range invalid {
		if _, err := decodeRequest(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input[:min(80, len(input))])
		}
	}
	for _, args := range [][]string{{"--json", "--name", "Bad"}, {"--install", ""}, {"--unknown"}, {"unexpected"}, {}} {
		output.Reset()
		diagnostics.Reset()
		if run(args, strings.NewReader("{}"), &output, &diagnostics) == 0 {
			t.Fatalf("accepted bad CLI: %v", args)
		}
		if !json.Valid(output.Bytes()) {
			t.Fatalf("invalid error JSON: %s", &output)
		}
	}
	output.Reset()
	diagnostics.Reset()
	if run([]string{"--help"}, nil, &output, &diagnostics) != 0 {
		t.Fatal("help failed")
	}
}

func TestManifestAndWrapperIntegrity(t *testing.T) {
	r := sampleRequest(t)
	p, err := loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	r, err = validate(r, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = renderFiles(r, p); err != nil {
		t.Fatal(err)
	}
	p.WrapperSHA = "bad"
	if _, err = renderFiles(r, p); err == nil {
		t.Fatal("unchecked wrapper checksum")
	}
	for _, name := range []string{"android_project_scaffold", "android_project_scaffold.exe"} {
		data, err := manifest(name)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Command []string `json:"command"`
		}
		if err = json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m.Command[0] != "{dir}/"+name || m.Command[1] != "--json" {
			t.Fatal("wrong manifest binary")
		}
	}
}
