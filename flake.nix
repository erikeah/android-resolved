{
  description = "android-resolved – systemd-resolved clone for Android with split DNS";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    inputs@{ nixpkgs, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ ];
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      perSystem =
        {
          config,
          self',
          inputs',
          pkgs,
          system,
          ...
        }:
        let
          buildToolsVersion = "37.0.0";
          androidsdkenv =
            (pkgs.androidenv.composeAndroidPackages {
              platformVersions = [ "34" "35" ];
              buildToolsVersions = [ "36.0.0" buildToolsVersion ];
              platformToolsVersion = [ "37.0.0" ];
              includeNDK = true;
            }).androidsdk;
        in
        {
          _module.args.pkgs = import nixpkgs {
              inherit system;
              config.android_sdk.accept_license = true;
              config.allowUnfree = true;
          };
          devShells.default = pkgs.mkShell rec {
            ANDROID_HOME = "${androidsdkenv}/libexec/android-sdk";
            ANDROID_NDK_ROOT = "${ANDROID_HOME}/ndk-bundle";
            ANDROID_SDK_ROOT = ANDROID_HOME;
            GRADLE_OPTS = "-Dorg.gradle.project.android.aapt2FromMavenOverride=${ANDROID_HOME}/build-tools/${buildToolsVersion}/aapt2";

            packages = with pkgs; [
              androidsdkenv
              bind
              go
              gomobile
              gopls
              gradle_9
              jdk21
            ];
          };

          apps.build-apk = {
            type = "app";
            program = pkgs.writeShellApplication {
              name = "build-apk";
              runtimeInputs = [ pkgs.go pkgs.gomobile pkgs.gradle_9 pkgs.jdk21 androidsdkenv ];
              text = ''
                set -euo pipefail
                cd "$(git rev-parse --show-toplevel)"

                echo "=== Verifying Go code ==="
                go vet ./mobile/... ./internal/...

                echo "=== Building AAR ==="
                AAR_SRC="app/app/libs/resolved.aar"
                AAR_WORK="$(mktemp -d)"
                rm -f "$AAR_SRC"
                NIX_BUILD_TOP="$AAR_WORK" GOPATH="$HOME/gomobile" gomobile bind \
                  -target=android -androidapi 34 \
                  -o "$AAR_SRC" -v ./mobile
                rm -rf "$AAR_WORK"

                echo "=== Building APK ==="
                ANDROID_HOME="$ANDROID_SDK_ROOT" \
                  gradle -p app assembleDebug

                echo "=== APK ready ==="
                ls -la app/app/build/outputs/apk/debug/app-debug.apk
              '';
            };
          };
        };
    };
}
