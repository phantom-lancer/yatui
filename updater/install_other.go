//go:build !windows

package updater

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

const linuxInstallScript = `#!/bin/sh
trap 'rm -f -- "$0"' EXIT
pid="$1"
target="$2"
staged="$3"
backup="$4"
shift 4

while kill -0 "$pid" 2>/dev/null; do
    sleep 0.15
done

if ! cp -p -- "$target" "$backup"; then
    "$target" "$@" &
    exit 1
fi
if ! mv -f -- "$staged" "$target"; then
    cp -p -- "$backup" "$target"
    rm -f -- "$backup"
    "$target" "$@" &
    exit 1
fi

"$target" "$@" &
new_pid=$!
sleep 5
if kill -0 "$new_pid" 2>/dev/null; then
    rm -f -- "$backup"
else
    cp -p -- "$backup" "$target"
    rm -f -- "$backup"
    "$target" "$@" &
fi

rm -f -- "$0"
`

// InstallCurrentExecutable supports Linux binaries. macOS packages are not
// published by the project, so they deliberately keep the manual update path.
func InstallCurrentExecutable(staged string, args []string) error {
	if runtime.GOOS != "linux" {
		return errors.New("automatic installation is currently supported on Linux and Windows only")
	}
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
	if filepath.Dir(target) != filepath.Dir(staged) {
		return errors.New("update file must be downloaded next to the executable")
	}
	if _, err := os.Stat(staged); err != nil {
		return err
	}

	script, err := os.CreateTemp("", "yatui-update-*.sh")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	if _, err := script.WriteString(linuxInstallScript); err != nil {
		script.Close()
		_ = os.Remove(scriptPath)
		return err
	}
	if err := script.Chmod(0700); err != nil {
		script.Close()
		_ = os.Remove(scriptPath)
		return err
	}
	if err := script.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}

	commandArgs := []string{
		scriptPath,
		strconv.Itoa(os.Getpid()),
		target,
		staged,
		target + ".yatui-backup",
	}
	commandArgs = append(commandArgs, args...)
	cmd := exec.Command("sh", commandArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}
