package config

import (
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr, DataDir string
	Lease         time.Duration
	Tick          time.Duration
}

func Default() Config {
	return Config{Addr: ":8080", DataDir: "./data", Lease: 5 * time.Minute, Tick: time.Second}
}

func FromEnv(base Config) Config {
	if v := os.Getenv("FLEETFORGE_ADDR"); v != "" {
		base.Addr = v
	}
	if v := os.Getenv("FLEETFORGE_DATA"); v != "" {
		base.DataDir = v
	}
	if v := os.Getenv("FLEETFORGE_LEASE"); v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			base.Lease = d
		}
	}
	if v := os.Getenv("FLEETFORGE_TICK"); v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			base.Tick = d
		}
	}
	return base
}

func Parse(args []string) (Config, string, error) {
	c := FromEnv(Default())
	fs := flag.NewFlagSet("fleetforge", flag.ContinueOnError)
	addr, data, lease, tick := c.Addr, c.DataDir, c.Lease.String(), c.Tick.String()
	mode := "serve"
	fs.StringVar(&addr, "addr", addr, "HTTP listen address")
	fs.StringVar(&data, "data", data, "durable data directory")
	fs.StringVar(&lease, "lease", lease, "agent lease duration")
	fs.StringVar(&tick, "tick", tick, "scheduler interval")
	fs.StringVar(&mode, "mode", mode, "serve or smoke")
	if err := fs.Parse(args); err != nil {
		return c, mode, err
	}
	ld, err := time.ParseDuration(lease)
	if err != nil {
		return c, mode, errors.New("invalid lease duration")
	}
	td, err := time.ParseDuration(tick)
	if err != nil {
		return c, mode, errors.New("invalid tick duration")
	}
	if strings.TrimSpace(data) == "" || ld <= 0 || td <= 0 {
		return c, mode, errors.New("invalid configuration")
	}
	c = Config{Addr: addr, DataDir: data, Lease: ld, Tick: td}
	return c, mode, nil
}

func (c Config) Validate() error {
	if c.Addr == "" || c.DataDir == "" || c.Lease <= 0 || c.Tick <= 0 {
		return errors.New("invalid configuration")
	}
	return nil
}

func (c Config) Values() map[string]string {
	return map[string]string{"addr": c.Addr, "data": c.DataDir, "lease": strconv.FormatInt(int64(c.Lease), 10), "tick": strconv.FormatInt(int64(c.Tick), 10)}
}
