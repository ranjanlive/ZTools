# Version 1.0.0 validation

Local validation completed on Windows on 2026-10-14.

- Go unit tests and vet passed; generator coverage was 79.4%.
- Actual zagent tools discovery and invocation passed using a temporary agent home:
  native installation, generation, existing-destination rejection, schema validation,
  and workspace containment. No model or real credentials were used.
- Official Gradle distribution and wrapper JAR checksums were verified.
- AGP 9.4.1 and the pinned dependency artifacts were available in official Maven
  repositories; AGP's bundled Kotlin was verified as 2.2.10.
- A generated project with XML characters, quotes, %, $, an apostrophe, Hindi text
  and emoji in the display name passed assembleDebug, assembleRelease, lintDebug,
  testDebugUnitTest and assembleDebugAndroidTest.
- Build environment: Android SDK Platform 37, Gradle 9.6.0, AGP 9.4.1, Android Studio
  bundled OpenJDK 25. The source Java compatibility is 17; CI separately tests JDK 17.
- Lint had no errors and one intentional version-availability warning: the Compose
  compiler remains at 2.2.10 to match AGP's bundled Kotlin rather than independently
  switching to 2.4.21.
- The release APK is unsigned. Application signing is intentionally left to the app
  developer. No generated APK is part of this tool's public binary release.

The release workflow runs native smoke tests on Windows, Linux and macOS, plus an
Android build on Linux/JDK 17 before packaging a draft release. Local cross-compilation
does not by itself establish native Linux/macOS runtime verification.

Instrumentation tests were compiled; they were not executed on a device/emulator.
No claim of device UI, splash rendering or runtime Android-version compatibility
testing is made. Run connectedDebugAndroidTest for that additional verification.
