package cached

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/PlakarKorp/plakar/appcontext"
	"golang.org/x/sys/windows"
)

func setupSyslog(ctx *appcontext.AppContext) error {
	ctx.GetLogger().SetOutput(io.Discard)
	return nil
}

func daemonize(argv []string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(binary, argv[1:]...)
	cmd.Env = append(os.Environ(), "REEXEC=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
		// is particularly enlightening.
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}

	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
