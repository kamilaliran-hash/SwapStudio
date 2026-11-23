// SwapStudio Setup: a per-user Windows installer (no admin rights needed).
//
// Installs to %LOCALAPPDATA%\Programs\SwapStudio, adds Start Menu and Desktop shortcuts,
// and registers an uninstaller under Settings > Apps. The same program is copied into the
// install folder as Uninstall.exe and removes everything when run.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed payload/SwapStudio.exe
var appExe []byte

const (
	appName    = "SwapStudio"
	appVersion = "1.0.0"
	publisher  = "SwapStudio"
	regKey     = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwapStudio`

	mbOK           = 0x0
	mbYesNo        = 0x4
	mbIconError    = 0x10
	mbIconQuestion = 0x20
	mbIconInfo     = 0x40
	idYes          = 6
)

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	messageBoxW = user32.NewProc("MessageBoxW")
)

func msgBox(text string, flags uintptr) int {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(appName + " Setup")
	r, _, _ := messageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
	return int(r)
}

func hidden(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func powershell(script string) error {
	return hidden("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).Run()
}

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(base, "Programs", appName)
}

func stopRunningApp() { _ = hidden("taskkill", "/IM", "SwapStudio.exe", "/F").Run() }

func install() {
	dir := installDir()
	if msgBox(fmt.Sprintf("Install %s %s?\n\nIt will be installed for your user account in:\n%s\n\nA Start Menu and a Desktop shortcut will be created.", appName, appVersion, dir),
		mbYesNo|mbIconQuestion) != idYes {
		return
	}
	stopRunningApp()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail("Could not create the install folder", err)
		return
	}
	target := filepath.Join(dir, "SwapStudio.exe")
	if err := os.WriteFile(target, appExe, 0o755); err != nil {
		fail("Could not copy SwapStudio.exe (is it still running?)", err)
		return
	}
	uninstaller := filepath.Join(dir, "Uninstall.exe")
	if self, err := os.Executable(); err == nil {
		if b, err := os.ReadFile(self); err == nil {
			_ = os.WriteFile(uninstaller, b, 0o755)
		}
	}

	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$w=New-Object -ComObject WScript.Shell
foreach($p in @([Environment]::GetFolderPath('Programs'),[Environment]::GetFolderPath('Desktop'))){
  $s=$w.CreateShortcut((Join-Path $p 'SwapStudio.lnk'))
  $s.TargetPath=%s; $s.WorkingDirectory=%s; $s.IconLocation=%s; $s.Description='Face and head swap'
  $s.Save()
}`, psQuote(target), psQuote(dir), psQuote(target+",0"))
	shortcutErr := powershell(script)

	reg := func(name, typ, val string) {
		_ = hidden("reg", "add", regKey, "/v", name, "/t", typ, "/d", val, "/f").Run()
	}
	reg("DisplayName", "REG_SZ", appName)
	reg("DisplayVersion", "REG_SZ", appVersion)
	reg("Publisher", "REG_SZ", publisher)
	reg("DisplayIcon", "REG_SZ", target)
	reg("InstallLocation", "REG_SZ", dir)
	reg("UninstallString", "REG_SZ", `"`+uninstaller+`" /uninstall`)
	reg("EstimatedSize", "REG_DWORD", fmt.Sprint(len(appExe)/1024))
	reg("NoModify", "REG_DWORD", "1")
	reg("NoRepair", "REG_DWORD", "1")

	note := ""
	if shortcutErr != nil {
		note = "\n\n(The shortcuts could not be created. You can start the app from the install folder.)"
	}
	if msgBox(appName+" is installed."+note+"\n\nStart it now?", mbYesNo|mbIconInfo) == idYes {
		_ = exec.Command(target).Start()
	}
}

func uninstall() {
	dir := installDir()
	if msgBox("Remove "+appName+" from this computer?", mbYesNo|mbIconQuestion) != idYes {
		return
	}
	stopRunningApp()
	_ = powershell(`foreach($p in @([Environment]::GetFolderPath('Programs'),[Environment]::GetFolderPath('Desktop'))){
  Remove-Item -LiteralPath (Join-Path $p 'SwapStudio.lnk') -ErrorAction SilentlyContinue }`)
	_ = hidden("reg", "delete", regKey, "/f").Run()
	_ = os.Remove(filepath.Join(dir, "SwapStudio.exe"))
	msgBox(appName+" has been removed.", mbOK|mbIconInfo)
	// Uninstall.exe can't delete itself while running: remove the folder a moment after exit.
	_ = hidden("cmd", "/C", "ping 127.0.0.1 -n 3 > nul & rmdir /S /Q \""+dir+"\"").Start()
}

func fail(what string, err error) {
	msgBox(what+".\n\n"+err.Error(), mbOK|mbIconError)
}

func main() {
	self, _ := os.Executable()
	isUninstaller := strings.EqualFold(filepath.Base(self), "Uninstall.exe")
	for _, a := range os.Args[1:] {
		if strings.EqualFold(a, "/uninstall") || strings.EqualFold(a, "--uninstall") {
			isUninstaller = true
		}
	}
	if isUninstaller {
		uninstall()
	} else {
		install()
	}
}
