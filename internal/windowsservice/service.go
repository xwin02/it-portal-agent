package windowsservice

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/itportal/it-portal-agent/internal/config"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

const (
	Name = "ITPortalAgent"
	DisplayName = "IT Portal Agent"
	Description = "Enterprise IT Inventory Agent"
)

type Handler struct { stop func() }

func (h Handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for request := range requests {
		switch request.Cmd {
		case svc.Interrogate:
			status <- request.CurrentStatus
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			if h.stop != nil { h.stop() }
			return false, 0
		}
	}
	return false, 0
}

func Run(stop func()) error {
	isService, err := svc.IsWindowsService()
	if err != nil { return fmt.Errorf("detect service context: %w", err) }
	handler := Handler{stop: stop}
	if isService { return svc.Run(Name, handler) }
	return debug.Run(Name, handler)
}

func Install() error {
	executable, err := os.Executable()
	if err != nil { return fmt.Errorf("resolve executable: %w", err) }
	absoluteExecutable, err := filepath.Abs(executable)
	if err != nil { return fmt.Errorf("resolve executable path: %w", err) }
	if _, err := config.LoadOrCreate(); err != nil { return err }
	if err := runSC("create", Name, "binPath=", `"`+absoluteExecutable+`"`, "start=", "auto", "DisplayName=", DisplayName); err != nil { return err }
	if err := runSC("description", Name, Description); err != nil { return err }
	return nil
}

func Uninstall() error { return runSC("delete", Name) }
func Start() error { return runSC("start", Name) }
func Stop() error { return runSC("stop", Name) }
func Restart() error { if err := Stop(); err != nil { return err }; return Start() }

func runSC(arguments ...string) error {
	output, err := exec.Command("sc.exe", arguments...).CombinedOutput()
	if err != nil { return fmt.Errorf("service command failed: %w: %s", err, string(output)) }
	return nil
}
