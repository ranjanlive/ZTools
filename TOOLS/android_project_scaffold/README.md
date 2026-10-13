# Android Project Scaffold

Version 1.0.0. A standalone, offline project generator for AI agents and ordinary
command-line automation. Creates a minimal Kotlin + Jetpack Compose Android app.
No Python, Node, Go, Java, Android SDK, or network connection is needed to generate
a project using a release binary. Building the generated app requires an Android
development environment.

This is an independent template inspired by Android Studio's Empty Activity,
adapted from the original workspace scaffolder. It does not invoke Android Studio
or claim to reproduce every IDE template option.

## Download and setup

Download from [ranjanlive/ZTools releases](https://github.com/ranjanlive/ZTools/releases).
Choose the release ZIP matching your OS and CPU: `windows`, `linux`, or `darwin`
(macOS), with `amd64` (Intel/AMD x64) or `arm64` (Apple Silicon/ARM64). ZIPs contain
one `android_project_scaffold/` directory. Verify its checksum against
`SHA256SUMS.txt`, then extract it. See [SETUP.md](SETUP.md) for full instructions.

Windows PowerShell:

```powershell
.\android_project_scaffold.exe --name MyApp --package com.example.myapp --output .\MyApp
.\android_project_scaffold.exe --install "C:\path\to\agent-home"
```

Linux/macOS:

```sh
chmod +x android_project_scaffold
./android_project_scaffold --name MyApp --package com.example.myapp --output ./MyApp
./android_project_scaffold --install /path/to/agent-home
```

The native installer copies its own executable, manifest, and license notices into
`<agent-home>/tools/android_project_scaffold/`. It refuses an existing installation.
It does not download anything or change engine settings. Restart the agent session
after installation. Alternatively, copy the extracted tool directory into the
agent home's `tools/` directory.

## Agent and automation contract

The packaged manifest works with ZAutomation Engine's third-party tool protocol.
Other automation clients can run the executable directly with `--json`: one JSON
arguments object on stdin, one JSON result object on stdout, exit 0 on success or
1 on failure. Diagnostics and CLI help go to stderr. No interactive prompts.

```json
{
  "name": "My App",
  "package": "com.example.myapp",
  "output": "apps/MyApp",
  "min_sdk": 26,
  "template": "empty_activity",
  "dry_run": false
}
```

Required: `name`, `package`, `output`. Optional: `min_sdk` (26â€“37; default 26),
`template` (currently only `empty_activity`), `sdk_path` (existing SDK directory),
`dry_run` (default false). Unknown keys and null values are rejected. Input is
limited to 64 KiB. Application IDs use lowercase ASCII segments and exclude Java/
Kotlin reserved identifiers. Display names support Unicode, punctuation and spaces.

`output` is relative to the process's current directory unless absolute. When
`ZAGENT_WORKSPACE` is set, generation must stay inside that workspace; existing
symlink ancestors are resolved before checking. The standalone CLI permits an
explicit destination elsewhere. Use a trusted workspace: this tool is not a
sandbox against another process maliciously replacing directories during writes.

Success returns `{"output":"<JSON report as text>","is_error":false,"report":{...}}`.
The engine reads `output`; standalone clients can read `report` directly. The report
includes absolute project directory, generated files, selected versions, minimum
SDK and next steps. Failure returns `{"output":"reason","is_error":true}`.

The tool reserves its destination exclusively. Existing files, directories, and
symlinks are refused, including empty directories. Concurrent agents targeting the
same directory get one successful invocation; use distinct destinations for
independent projects. Generation rollback removes only files and directories made
by that call. Newly created ancestor directories may remain after a failure.

## Generated project

- One `app` module, Kotlin DSL, a pinned version catalog, official Gradle wrapper.
- Compose + Material 3, light/dark and dynamic themes, edge-to-edge with insets.
- Minimal screen showing the app name, splash screen, adaptive/monochrome icons.
- JVM test, app-context instrumentation test, and a Compose UI test.
- Release shrinking with an app-specific ProGuard rules file.
- `.scaffold.json` records generator/profile metadata and is ignored by Git.
- `local.properties` is written only when `sdk_path` is supplied and is ignored.

The Gradle project name is the last application ID segment (e.g. `myapp`), while
the Android display name retains the requested text. This avoids Gradle's name
restrictions without compromising the display name.

No navigation, database, dependency injection, analytics, network permissions,
signing keys, or multi-module structure is added automatically. Add those according
to the application's requirements.

## Pinned stable profile

| Component | Version |
|---|---|
| Android compile / target SDK | 37 / 37 |
| Minimum SDK | 26 by default; accepts 26â€“37 |
| Android Gradle plugin | 9.4.1 |
| Gradle | 9.6.0 |
| Built-in Kotlin / Compose compiler plugin | 2.2.10 / 2.2.10 |
| Compose BOM | 2026.09.00 |
| Core KTX / Activity Compose / Lifecycle | 1.19.1 / 1.13.0 / 2.11.0 |
| Build JDK | 17 minimum; Java source/target compatibility 17 |

These are intentional release pins, not an automatic promise of the newest
versions. The AGP POM's Kotlin Gradle plugin dependency was checked before matching
the Compose compiler. Updates require verification and a new release; generation
never fetches version metadata.

[AGP compatibility](https://developer.android.com/build/releases/agp-9-4-0-release-notes),
[built-in Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin),
[Compose BOM](https://developer.android.com/develop/ui/compose/bom),
[Gradle wrapper verification](https://docs.gradle.org/current/userguide/gradle_wrapper.html).

## Build this tool from source

Only Go 1.26+ is required for development. There are no external Go modules.
Run from this directory:

```sh
go test ./...
go vet ./...
go build -trimpath -o android_project_scaffold .
go run ./cmd/package --version 1.0.0
```

On Windows, build with `-o android_project_scaffold.exe`. The source manifest is a
platform-neutral template; `--install` and release packaging generate a manifest
pointing at the correct executable filename.

To run real-engine integration, build this tool and `zagent`, set
`SCAFFOLD_TEST_BINARY` and `SCAFFOLD_TEST_AGENT` to their absolute paths, then run
`go test -run TestRealEngineIntegration -v .`. It uses temporary agent homes,
requires no model or API key, and tests discovery, generation, installation
conflicts, schema rejection, and workspace containment.

See [RELEASE.md](RELEASE.md) for packaging and publication. Apache-2.0;
see [LICENSE](LICENSE) and [NOTICE](NOTICE).
