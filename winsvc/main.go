//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
)

const (
	svcName = "CartenzMonitor"
	appDir  = `D:\cartenz\cartenz-monitor-2.1`
)

type handler struct{}

func startChild() (*exec.Cmd, chan error, error) {
	child := exec.Command(filepath.Join(appDir, "cartenzmon2.exe"), "daemon", "-l", "127.0.0.1", "-p", "31337")
	child.Env = append(os.Environ(),
		`SLIVER_ROOT_DIR=C:\Windows\System32\config\systemprofile\.sliver`,
		`SLIVER_ASSETS_DIR=`+filepath.Join(appDir, "assets"),
	)
	if err := child.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	return child, done, nil
}

func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}

	child, done, err := startChild()
	if err != nil {
		return false, 1
	}
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				if child.Process != nil {
					_ = child.Process.Kill()
				}
				select {
				case <-done:
				case <-time.After(15 * time.Second):
				}
				return false, 0
			}
		case <-done:
			return false, 0
		}
	}
}

func main() {
	if inService, err := svc.IsWindowsService(); err == nil && inService {
		_ = svc.Run(svcName, &handler{})
		return
	}
	child, done, err := startChild()
	if err != nil {
		os.Exit(1)
	}
	<-done
	_ = child
}
