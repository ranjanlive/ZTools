# ZTools

Tools for AI agents and automation projects, maintained in the ranjanlive/ZTools repository.

## Android Project Scaffold

Version 1.0.0 creates a minimal Kotlin + Jetpack Compose Android project offline.
Download a native binary for Windows, Linux or macOS, on x64 or ARM64. Generation
requires no Python, Node, Go, Android Studio, Java, or network access. Building the
generated app requires an Android development environment.

- [Download releases](https://github.com/ranjanlive/ZTools/releases)
- [Tool documentation](TOOLS/android_project_scaffold/README.md)
- [Setup and engine integration](TOOLS/android_project_scaffold/SETUP.md)
- [Validation and limitations](TOOLS/android_project_scaffold/VALIDATION.md)
- [Release maintenance](TOOLS/android_project_scaffold/RELEASE.md)

Each tool lives in `TOOLS/<tool-name>/` with a matching JSON manifest. The Android
scaffolder's native `--install <agent-home>` command installs it into ZAutomation
Engine. Any automation client can also invoke its bounded JSON stdin/stdout API.

## Development

With Go 1.26+, from `TOOLS/android_project_scaffold`:

```sh
go test ./...
go vet ./...
go run ./cmd/package --version 1.0.0
```

Native tests run on Windows, Linux and macOS. The Android CI job builds debug and
release APKs, runs lint and JVM tests, and compiles instrumentation tests.
Tagging `android-scaffold-v1.0.0` runs validation and prepares a draft GitHub release.

Apache-2.0. Embedded Gradle wrapper notices are retained; generated application
code can use the developer's chosen license.
