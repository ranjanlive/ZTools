# Setup guide

## 1. Choose and verify a binary

Download the version's ZIP and `SHA256SUMS.txt` from the same
[ZTools release](https://github.com/ranjanlive/ZTools/releases).
Choose Windows x64/ARM64, Linux x64/ARM64, or macOS Intel/Apple Silicon. The binary
must match the OS and CPU; very old operating systems may not support the Go runtime.

Windows:

```powershell
Get-FileHash .\android_project_scaffold-1.0.0-windows-amd64.zip -Algorithm SHA256
Expand-Archive .\android_project_scaffold-1.0.0-windows-amd64.zip -DestinationPath .\scaffold-tool
cd .\scaffold-tool\android_project_scaffold
.\android_project_scaffold.exe --version
```

Compare the hash to the matching line in `SHA256SUMS.txt`. On Linux/macOS use
`sha256sum` / `shasum -a 256` to calculate it, extract the matching ZIP, then:

```sh
cd scaffold-tool/android_project_scaffold
chmod +x android_project_scaffold
./android_project_scaffold --version
```

The initial public binaries are unsigned. A checksum detects accidental changes;
it does not independently authenticate the publisher. Obtain binaries from the
maintainer's release page. Follow your organization's policy for unsigned tools;
do not disable operating-system security globally.

## 2. Install in ZAutomation Engine

Choose the home passed to `zagent --home`, or the home configured for the agent in
zsched. An agent home is distinct from the application's workspace and this source
repository's `TOOLS/` catalogue.

```powershell
.\android_project_scaffold.exe --install "C:\path\to\agent-home"
zagent tools list --home "C:\path\to\agent-home"
```

Linux/macOS:

```sh
./android_project_scaffold --install /path/to/agent-home
zagent tools list --home /path/to/agent-home
```

The installed tool should appear as `external`. Start a new agent session. If your
engine uses a tool allowlist, add `android_project_scaffold` to that agent's enabled
tools. Installation does not alter your allowlist or provider configuration.

Suggested agent request:

> Use android_project_scaffold to create a minimal Compose app named My App,
> application ID com.example.myapp, in apps/MyApp inside the workspace. Report
> the selected toolchain and next build commands.

For upgrades, keep the existing installation until current calls have finished.
Rename its tool folder out of `<home>/tools/` to a backup location, install the new
release, and start a new session. The installer never overwrites an installation.

## 3. Use without the engine

Windows CLI:

```powershell
.\android_project_scaffold.exe --name MyApp --package com.example.myapp --output "C:\work\My App"
.\android_project_scaffold.exe --name MyApp --package com.example.myapp --output "C:\work\Another App" --dry-run
.\android_project_scaffold.exe --profile
```

For reliable special-character handling across shells, write `request.json` as
UTF-8 (UTF-8 BOM is also accepted):

```json
{"name":"Fish & Chips","package":"com.example.fish","output":"apps/FishApp"}
```

PowerShell 7:

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
Get-Content -Raw -Encoding UTF8 .\request.json | .\android_project_scaffold.exe --json
```

Windows PowerShell 5.1 must also set `$OutputEncoding` as above to preserve Unicode.
Linux/macOS:

```sh
./android_project_scaffold --json < request.json
```

Automation libraries should pass JSON directly to stdin and parse stdout, avoiding
shell-quoted JSON. The tool does not require a shell.

## 4. Build the generated Android app

Generation is offline and requires no Android tools. To build, install a supported
Android Studio, use JDK 17 or newer supported by Gradle 9.6, and install SDK Platform
37 and Build Tools 36.0.0. Android Studio's SDK Manager is the easiest route.
If using SDK command-line tools:

```sh
sdkmanager "platforms;android-37.0" "build-tools;36.0.0" "platform-tools"
```

Review and accept Android SDK licenses through the normal SDK setup flow. Set
`ANDROID_HOME`, open the project in Android Studio, or supply `--sdk-path` during
generation. The generator does not search your personal directories for an SDK.

From the generated project directory:

```powershell
.\gradlew.bat assembleDebug lintDebug testDebugUnitTest assembleDebugAndroidTest
```

Linux/macOS:

```sh
chmod +x gradlew
./gradlew assembleDebug lintDebug testDebugUnitTest assembleDebugAndroidTest
```

The debug APK is under `app/build/outputs/apk/debug/`. These commands compile UI
instrumentation tests but do not execute them. With a suitable connected emulator
or device, run `connectedDebugAndroidTest`. Emulator creation and app installation
are separate operations.

The first Android build downloads Gradle and dependencies. Later offline builds
require all necessary artifacts to be cached; an offline scaffold does not make
the Android build dependency-free.

## Troubleshooting

| Problem | Resolution |
|---|---|
| Destination already exists | Choose a new directory. Even empty destinations are refused. |
| Tool not listed | Verify agent home and allowlist, restart the session, and check the packaged manifest/executable pair. |
| Outside ZAGENT_WORKSPACE | Choose an output inside the agent's workspace, including resolved symlinks. |
| min_sdk rejected | This profile accepts 26–37; lower API support needs a separately tested profile. |
| Gradle download/checksum failure | Check connectivity/proxy and the official pinned distribution. Do not remove checksum verification. |
| SDK missing | Install Platform 37, configure ANDROID_HOME, or supply sdk_path. |
| Java incompatible | Configure an appropriate Gradle build JDK; source compatibility alone does not select the daemon JDK. |
| macOS/Linux executable permission | Run chmod +x on the extracted executable and generated gradlew. |

[Android command-line builds](https://developer.android.com/build/building-cmdline),
[AGP requirements](https://developer.android.com/build/releases/agp-9-4-0-release-notes).
