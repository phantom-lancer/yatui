//go:build windows

package updater

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type installPayload struct {
	PID    int      `json:"pid"`
	Target string   `json:"target"`
	Staged string   `json:"staged"`
	Args   []string `json:"args"`
}

const installScript = `param([string]$Payload)
$ErrorActionPreference = 'Stop'
$scriptPath = $MyInvocation.MyCommand.Path
$data = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Payload)) | ConvertFrom-Json
$backup = $data.Target + '.yatui-backup'
$arguments = @()
foreach ($arg in $data.Args) {
    $value = [string]$arg
    if ($value -match '[\s"]') {
        $arguments += '"' + $value.Replace('"', '\"') + '"'
    } else {
        $arguments += $value
    }
}
try {
    try {
        $parent = [Diagnostics.Process]::GetProcessById([int]$data.PID)
        $parent.WaitForExit()
        $parent.Dispose()
    } catch [ArgumentException] {
    }

    try {
        [IO.File]::Replace($data.Staged, $data.Target, $backup, $true)
    } catch {
        Copy-Item -LiteralPath $data.Target -Destination $backup -Force
        Move-Item -LiteralPath $data.Staged -Destination $data.Target -Force
    }

    $updated = Start-Process -FilePath $data.Target -ArgumentList $arguments -PassThru -NoNewWindow
    Start-Sleep -Seconds 5
    if ($updated.HasExited) {
        throw 'Updated YATUI exited during startup'
    }
    Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
} catch {
    if (Test-Path -LiteralPath $backup) {
        Copy-Item -LiteralPath $backup -Destination $data.Target -Force
    }
    try {
        Start-Process -FilePath $data.Target -ArgumentList $arguments -NoNewWindow | Out-Null
    } catch {
    }
} finally {
    Remove-Item -LiteralPath $data.Staged -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 250
    Remove-Item -LiteralPath $scriptPath -Force -ErrorAction SilentlyContinue
}
`

// InstallCurrentExecutable starts a detached updater which waits for the
// current YATUI process to exit, replaces only its executable, and restarts it.
// The config, cache, Last.fm queue and all user data remain untouched.
func InstallCurrentExecutable(staged string, args []string) error {
	target, err := os.Executable()
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	staged, err = filepath.Abs(staged)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(target), filepath.Dir(staged)) {
		return errors.New("update file must be downloaded next to the executable")
	}
	if _, err := os.Stat(staged); err != nil {
		return err
	}

	payload, err := json.Marshal(installPayload{
		PID:    os.Getpid(),
		Target: target,
		Staged: staged,
		Args:   args,
	})
	if err != nil {
		return err
	}

	script, err := os.CreateTemp("", "yatui-update-*.ps1")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	if _, err := script.WriteString(installScript); err != nil {
		script.Close()
		_ = os.Remove(scriptPath)
		return err
	}
	if err := script.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(payload)
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", scriptPath,
		"-Payload", encoded,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}
