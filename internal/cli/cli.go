package cli

import (
	"errors"
	"flag"
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/service"
	"fleetforge/internal/store"
	"fmt"
	"strings"
)

type Options struct {
	Version    string
	Devices    []string
	Batch      int
	Concurrent int
	Retries    int
	Rollback   bool
}

func Parse(args []string) Options {
	f := flag.NewFlagSet("fleetforge", flag.ContinueOnError)
	v := f.String("version", "", "firmware version")
	d := f.String("devices", "", "comma separated")
	b := f.Int("batch-size", 1, "batch size")
	c := f.Int("max-concurrent", 1, "maximum active tasks")
	r := f.Int("retries", 1, "retry limit")
	rb := f.Bool("auto-rollback", false, "rollback after failures")
	_ = f.Parse(args)
	var ds []string
	if *d != "" {
		ds = strings.Split(*d, ",")
	}
	return Options{Version: *v, Devices: ds, Batch: *b, Concurrent: *c, Retries: *r, Rollback: *rb}
}

type Command struct {
	Name    string
	Options Options
	DataDir string
	Agent   string
	TaskID  string
}

// SmokeResult is the stable, user-facing summary produced by the smoke flow.
type SmokeResult struct {
	Completed int
	Status    domain.ReleaseStatus
}

func ParseCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, errors.New("command required")
	}
	c := Command{Name: args[0], DataDir: "./data"}
	fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
	fs.StringVar(&c.DataDir, "data", c.DataDir, "durable data directory")
	fs.StringVar(&c.Agent, "agent", "", "agent identity")
	fs.StringVar(&c.TaskID, "task", "", "task id")
	if c.Name == "create-release" {
		fs.StringVar(&c.Options.Version, "version", "", "firmware version")
		var devices string
		fs.StringVar(&devices, "devices", "", "comma separated")
		fs.IntVar(&c.Options.Batch, "batch-size", 1, "batch size")
		fs.IntVar(&c.Options.Concurrent, "max-concurrent", 1, "maximum active tasks")
		fs.IntVar(&c.Options.Retries, "retries", 1, "retry limit")
		fs.BoolVar(&c.Options.Rollback, "auto-rollback", false, "rollback after failures")
		if err := fs.Parse(args[1:]); err != nil {
			return c, err
		}
		if devices != "" {
			c.Options.Devices = strings.Split(devices, ",")
		}
		return c, nil
	}
	if err := fs.Parse(args[1:]); err != nil {
		return c, err
	}
	return c, nil
}

func Execute(c Command, s *service.Service) (any, error) {
	switch c.Name {
	case "smoke":
		return RunSmoke(c.DataDir)
	case "create-release":
		if c.Options.Version == "" {
			return nil, errors.New("version is required")
		}
		return s.CreateRelease(c.Options.Version, c.Options.Devices, c.Options.Batch, c.Options.Concurrent, c.Options.Retries, domain.RollbackPolicy{Auto: c.Options.Rollback, MaxFailures: 0})
	case "audit":
		return s.AuditSummary(), nil
	case "health":
		return s.RecoveryInfo(), nil
	default:
		return nil, errors.New("unknown command: " + c.Name)
	}
}

// RunSmoke exercises the public service workflow and verifies it survives a
// fresh DurableStore/Service construction before returning its summary.
func RunSmoke(dataDir string) (SmokeResult, error) {
	if strings.TrimSpace(dataDir) == "" {
		return SmokeResult{}, errors.New("data directory is required")
	}
	st := store.NewDurableStore(dataDir)
	s := service.New(st, ports.RealClock{}, &ports.SequenceID{})
	r, err := s.CreateRelease("2.0.0", []string{"smoke-device"}, 1, 1, 1, domain.RollbackPolicy{Auto: true, MaxFailures: 1})
	if err != nil {
		return SmokeResult{}, fmt.Errorf("create release: %w", err)
	}
	if _, err = s.RegisterDevice("smoke-device", "1.0.0", []string{"smoke"}); err != nil {
		return SmokeResult{}, fmt.Errorf("register device: %w", err)
	}
	t, err := s.Claim("smoke-device", "smoke-agent")
	if err != nil {
		return SmokeResult{}, fmt.Errorf("claim task: %w", err)
	}
	if err = s.StartTask(t.ID, "smoke-agent"); err != nil {
		return SmokeResult{}, fmt.Errorf("start task: %w", err)
	}
	if err = s.ReportPreflight(t.ID, true, ""); err != nil {
		return SmokeResult{}, fmt.Errorf("confirm preflight: %w", err)
	}
	if err = s.ConfirmInstall(t.ID, "smoke-agent"); err != nil {
		return SmokeResult{}, fmt.Errorf("complete task: %w", err)
	}
	// Constructing a new service is the recovery boundary being exercised.
	recovered := service.New(store.NewDurableStore(dataDir), ports.RealClock{}, &ports.SequenceID{})
	recoveredRelease, ok := recovered.State.Releases[r.ID]
	if !ok {
		return SmokeResult{}, errors.New("recover release: release not found")
	}
	if recoveredRelease.Completed != 1 || recoveredRelease.Status != domain.StatusCompleted {
		return SmokeResult{}, fmt.Errorf("recover release: completed=%d status=%s", recoveredRelease.Completed, recoveredRelease.Status)
	}
	return SmokeResult{Completed: recoveredRelease.Completed, Status: recoveredRelease.Status}, nil
}
