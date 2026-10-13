# Release and maintenance

## Build release artifacts

From this module directory, with Go 1.26+:

```sh
go test ./...
go vet ./...
go run ./cmd/package --version 1.0.0
```

The packager builds with CGO disabled and trimpath, creates six platform ZIPs plus
a source ZIP, and writes `dist/1.0.0/SHA256SUMS.txt`. Each binary ZIP includes the
matching manifest, README, setup guide, release guide, changelog, validation notes,
license and notice.
All generation templates and wrapper files are embedded in the executable.

The source archive includes only known source roots/files; `.verification`, local
binaries, release artifacts, and unrelated root files are excluded. Keep private
material out of `templates`, `profiles` and `cmd` as those are published source.
The packager refuses an existing release directory. Preserve released artifacts;
use a new version for changed contents.

## Release gates

1. Verify wrapper JAR and distribution hashes against official Gradle checksums.
2. Verify every pinned Maven artifact and AGP/Kotlin/Compose compatibility.
3. Run Go tests, vet, real-engine integration, and platform executable smoke tests.
4. Build a generated app: assembleDebug, lintDebug, testDebugUnitTest,
   assembleDebugAndroidTest. Include Unicode and punctuation in a display-name case.
5. Run connectedDebugAndroidTest on a supported device/emulator before advertising
   device-tested behavior. This is separate from compiling instrumentation tests.
6. Verify ZIP inventory, manifests, executable permissions and SHA256SUMS.txt.
7. Publish the exact reviewed ZIPs, source ZIP, checksums and validation notes.

`.github/workflows/android-scaffold.yml` runs cross-platform native tests and an
Android build on pull requests and manual dispatch. `release-android-scaffold.yml`
builds artifacts on `android-scaffold-v*` tags, but creates a **draft** GitHub release
for final review. Source workflow paths assume this tool remains in the engine
repository at `TOOLS/android_project_scaffold`.

Public repository: [ranjanlive/ZTools](https://github.com/ranjanlive/ZTools).
Local packaging does not publish a release. With these sources/workflows in that
repository, verify CI results, then tag the reviewed commit:

```sh
git tag android-scaffold-v1.0.0
git push origin android-scaffold-v1.0.0
```

Do not tag until the repository's release gates pass. The draft release workflow
will attach its build artifacts; review the draft before publishing. Public binaries
are unsigned initially; checksums accompany them. Add signing and provenance once
the repository and signing identity have been configured.

## Updating the profile

Update `profiles/stable.json` as a tested set, not individual latest versions.
Match the Compose compiler to AGP's actual resolved Kotlin compiler version. If
raising Kotlin above AGP's bundled version, explicitly configure and test that
upgrade. Keep preview dependencies out of the stable profile.

For a Gradle upgrade, generate the wrapper with the selected official Gradle
distribution, verify JAR/distribution hashes, retain the upstream notices, and copy
the scripts/JAR/properties into `templates/empty_activity/`. Preserve LF in gradlew,
CRLF in gradlew.bat, and executable permissions in release ZIPs.

Update README, changelog, default version, profile and tests together. A minor/patch
release must not silently modify an already published ZIP.
