package ajean

// Portable supervisor for the test copy, behind the official start/stop and
// preset handlers. Only process handles created by THIS app are controlled:
// no global service, foreign PID file, port occupant or API engine is adopted.
import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type parallelEngineProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // published by closing done
}

var parallelEngine struct {
	sync.Mutex
	control sync.Mutex
	p       *parallelEngineProcess
}

func parallelEngineLogPath() string { return filepath.Join(AjeanHome(), "ajean-embedding-test.log") }

func parallelEngineActive() bool {
	parallelEngine.Lock()
	defer parallelEngine.Unlock()
	if parallelEngine.p == nil {
		return false
	}
	select {
	case <-parallelEngine.p.done:
		return false
	default:
		return true
	}
}

func parallelEngineLogTail(n int) string {
	b, _ := os.ReadFile(parallelEngineLogPath())
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func parallelServiceAction(action string) error {
	parallelEngine.control.Lock()
	defer parallelEngine.control.Unlock()
	if action == "status" {
		fmt.Printf("moteur du TEST actif : %v\n", parallelEngineActive())
		return nil
	}
	if action != "start" && action != "stop" && action != "restart" {
		return fmt.Errorf("action indisponible pour le moteur portable : %s", action)
	}
	if action == "stop" || action == "restart" || isExternalConfig(ReadConfig()) {
		if err := stopParallelEngine(); err != nil {
			return err
		}
	}
	if action == "stop" || isExternalConfig(ReadConfig()) {
		return nil
	}
	if parallelEngineActive() {
		return nil
	}
	if _, err := normalizeParallelConfig(ReadConfig()); err != nil {
		return err
	}
	// La config doit porter le port que le moteur va réellement écouter. Sans
	// ça, un dossier sans preset importé a PORT vide : le moteur est lancé avec
	// --port 18080 mais LLMPort() répond 8080, et tout ce qui parle au moteur en
	// local (sonde, pont, page Moteur) frappe une porte vide.
	if err := ensureParallelEnginePort(); err != nil {
		return err
	}
	if err := preflightEngine(); err != nil {
		return err
	}
	port := parallelLocalEnginePort()
	if err := waitPortFree("127.0.0.1", port, 0); err != nil {
		return fmt.Errorf("port moteur %s occupé : aucun processus arrêté ; %w", port, err)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(parallelEngineLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	cmd := hideCmd(exec.Command(self, "serve"))
	cmd.Dir = AjeanHome()
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = log, log
	prepareBoundChild(cmd)
	if err = cmd.Start(); err != nil {
		log.Close()
		return err
	}
	bindChild(cmd)
	p := &parallelEngineProcess{cmd: cmd, done: make(chan struct{})}
	parallelEngine.Lock()
	parallelEngine.p = p
	parallelEngine.Unlock()
	go func() { p.err = cmd.Wait(); log.Close(); close(p.done) }()
	select {
	case <-p.done:
		return fmt.Errorf("le moteur du TEST s’est arrêté (%v) : %s", p.err, parallelEngineLogTail(12))
	case <-time.After(300 * time.Millisecond):
		return nil // loading, not proof of readiness
	}
}

// Caller holds control. Never read a PID from disk or stop an occupied port.
func stopParallelEngine() error {
	parallelEngine.Lock()
	p := parallelEngine.p
	parallelEngine.Unlock()
	if p == nil {
		return nil
	}
	select {
	case <-p.done:
	default:
		// Process.Kill uses the owned OS handle. The serve child's bound job
		// object (Windows) also closes its llama-server subtree on termination.
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		select {
		case <-p.done:
		case <-time.After(10 * time.Second):
			return fmt.Errorf("arrêt du moteur TEST trop long")
		}
	}
	parallelEngine.Lock()
	if parallelEngine.p == p {
		parallelEngine.p = nil
	}
	parallelEngine.Unlock()
	return waitPortFree("127.0.0.1", parallelLocalEnginePort(), 15*time.Second)
}
