# Agent Conventions

## Build
- Use `nix run .#build-apk` (not manual gradle/gomobile commands)
- This runs: go vet → gomobile bind AAR → gradle assembleDebug
- AAR is written to `app/app/libs/resolved.aar`
- APK output: `app/app/build/outputs/apk/debug/app-debug.apk`

## Installation & Testing
- Install via adb: `adb install -r app/app/build/outputs/apk/debug/app-debug.apk`
- Client install: same path with `app/client/build/outputs/apk/debug/client-debug.apk`
- Service tag: `com.androidresolved`, Client tag: `com.androidresolved.client`

## Go/javapkg
- gomobile bind uses `-javapkg resolved` flag
- Import path in Kotlin: `resolved.resolved.Resolved`
