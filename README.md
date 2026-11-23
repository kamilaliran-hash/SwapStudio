# SwapStudio

A face and head swap app with accessories. Everything runs on the device, and photos are never uploaded.

- `app/src/main/assets/index.html`: the whole app (HTML + JavaScript). Edit it here.
- `app/`: the Android wrapper (a WebView with a photo picker and save-to-gallery).
- `desktop/`: the Windows launcher (Go). It serves the app locally and opens it in your browser.
- `installer/`: the Windows installer (Go). It installs per user with no admin rights, adds Start Menu and Desktop shortcuts, and registers an uninstaller under Settings › Apps.

## Build the APK

**Option A: Android Studio.** Open this folder, wait for Gradle sync, then choose
*Build › Build App Bundle(s) / APK(s) › Build APK(s)*. Or run `./gradlew assembleRelease`.
The APK is in `app/build/outputs/apk/release/`.

**Option B: GitHub (no local setup).** Push this folder to a GitHub repo. The included workflow
(`.github/workflows/build.yml`) builds both the APK and the EXE. Download them from the run's
*Artifacts* section on the repo's **Actions** tab.

The workflow also bundles the face-point library (`clmtrackr.min.js`), so auto-detect works offline.
For a local build, download it into `app/src/main/assets/` yourself:
https://cdn.jsdelivr.net/npm/clmtrackr@1.1.2/build/clmtrackr.min.js
Without it, the app loads the library from the internet. The 3-tap manual mode always works.

The release APK is signed with the debug key so it installs directly.
Before publishing to Google Play, switch to your own keystore.

## Build the EXE and the installer

```
python3 desktop/tools/make_syso.py desktop/SwapStudio.ico desktop/rsrc_windows_amd64.syso
python3 desktop/tools/make_syso.py desktop/SwapStudio.ico installer/rsrc_windows_amd64.syso
cp app/src/main/assets/index.html desktop/web/
(cd desktop && GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui" -o SwapStudio.exe .)
mkdir -p installer/payload && cp desktop/SwapStudio.exe installer/payload/
(cd installer && GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui" -o SwapStudio-Setup.exe .)
```
