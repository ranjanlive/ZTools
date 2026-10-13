// SPDX-License-Identifier: Apache-2.0
// Build platform ZIPs and a source ZIP from this module; no packaging dependencies.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type entry struct {
	Name string
	Data []byte
	Mode fs.FileMode
}

func writeZip(path string, entries []entry) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: "android_project_scaffold/" + e.Name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
		h.SetMode(e.Mode)
		w, e2 := z.CreateHeader(h)
		if e2 != nil {
			_ = z.Close()
			_ = f.Close()
			return e2
		}
		if _, e2 = w.Write(e.Data); e2 != nil {
			_ = z.Close()
			_ = f.Close()
			return e2
		}
	}
	err = z.Close()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func build() error {
	version := flag.String("version", "1.0.0", "Release version")
	flag.Parse()
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$`).MatchString(*version) {
		return fmt.Errorf("invalid release version")
	}
	destination := filepath.Join("dist", *version)
	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		return fmt.Errorf("refusing existing release directory: %w", err)
	}
	documentation := []string{"README.md", "SETUP.md", "RELEASE.md", "CHANGELOG.md", "VALIDATION.md", "LICENSE", "NOTICE"}
	var docs []entry
	for _, name := range documentation {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		docs = append(docs, entry{name, data, 0644})
	}
	manifest, err := os.ReadFile("android_project_scaffold.json")
	if err != nil {
		return err
	}
	var checksums []string
	for _, platform := range []struct{ OS, Arch string }{{"windows", "amd64"}, {"windows", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}} {
		name := "android_project_scaffold"
		if platform.OS == "windows" {
			name += ".exe"
		}
		binPath := filepath.Join(destination, name+"-"+platform.OS+"-"+platform.Arch)
		cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", "-s -w -X main.version="+*version, "-o", binPath, ".")
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "GOOS=") && !strings.HasPrefix(value, "GOARCH=") && !strings.HasPrefix(value, "CGO_ENABLED=") {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS="+platform.OS, "GOARCH="+platform.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
		binary, err := os.ReadFile(binPath)
		if err != nil {
			return err
		}
		var m map[string]any
		if err = json.Unmarshal(manifest, &m); err != nil {
			return err
		}
		m["command"] = []string{"{dir}/" + name, "--json"}
		m["platforms"] = []string{platform.OS}
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		entries := append([]entry{}, docs...)
		entries = append(entries, entry{name, binary, 0755}, entry{"android_project_scaffold.json", append(data, '\n'), 0644})
		zipName := "android_project_scaffold-" + *version + "-" + platform.OS + "-" + platform.Arch + ".zip"
		if err = writeZip(filepath.Join(destination, zipName), entries); err != nil {
			return err
		}
		if err = os.Remove(binPath); err != nil {
			return err
		}
		archive, err := os.ReadFile(filepath.Join(destination, zipName))
		if err != nil {
			return err
		}
		checksums = append(checksums, fmt.Sprintf("%x  %s", sha256.Sum256(archive), zipName))
		fmt.Println("Built", zipName)
	}
	var source []entry
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			allowed := path == "." || path == "cmd" || strings.HasPrefix(filepath.ToSlash(path), "cmd/") || path == "templates" || strings.HasPrefix(filepath.ToSlash(path), "templates/") || path == "profiles" || strings.HasPrefix(filepath.ToSlash(path), "profiles/")
			if !allowed {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path)
		allowed := strings.HasPrefix(rel, "templates/") || strings.HasPrefix(rel, "profiles/") || strings.HasPrefix(rel, "cmd/")
		for _, name := range append(documentation, "go.mod", "main.go", "main_test.go", "integration_test.go", "android_project_scaffold.json", ".gitignore") {
			if rel == name {
				allowed = true
			}
		}
		if !allowed {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source archive refuses symlink %s", path)
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		mode := fs.FileMode(0644)
		if d.Name() == "gradlew" {
			mode = 0755
		}
		source = append(source, entry{rel, data, mode})
		return nil
	})
	if err != nil {
		return err
	}
	sourceName := "android_project_scaffold-" + *version + "-source.zip"
	if err = writeZip(filepath.Join(destination, sourceName), source); err != nil {
		return err
	}
	archive, err := os.ReadFile(filepath.Join(destination, sourceName))
	if err != nil {
		return err
	}
	checksums = append(checksums, fmt.Sprintf("%x  %s", sha256.Sum256(archive), sourceName))
	sort.Strings(checksums)
	if err = os.WriteFile(filepath.Join(destination, "SHA256SUMS.txt"), []byte(strings.Join(checksums, "\n")+"\n"), 0644); err != nil {
		return err
	}
	fmt.Println("Release files:", destination)
	return nil
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
