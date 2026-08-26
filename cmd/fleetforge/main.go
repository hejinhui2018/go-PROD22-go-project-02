package main

import (
	"encoding/json"
	"flag"
	"fleetforge/internal/cli"
	"fleetforge/internal/config"
	"fleetforge/internal/domain"
	"fleetforge/internal/httpapi"
	"fleetforge/internal/ports"
	"fleetforge/internal/service"
	"fleetforge/internal/store"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	// Subcommands are parsed before global flags so `smoke --data DIR` works.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cmd, err := cli.ParseCommand(os.Args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if cmd.Name == "smoke" {
			result, err := cli.RunSmoke(cmd.DataDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, "smoke failed:", err)
				os.Exit(1)
			}
			fmt.Printf("completed=%d status=%s\n", result.Completed, result.Status)
			return
		}
		st := store.NewDurableStore(cmd.DataDir)
		s := service.New(st, ports.RealClock{}, &ports.SequenceID{})
		v, err := cli.Execute(cmd, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	mode := flag.String("mode", "serve", "serve or smoke")
	data := flag.String("data", "./data", "data dir")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()
	_ = config.Default()
	st := store.NewDurableStore(*data)
	s := service.New(st, ports.RealClock{}, &ports.SequenceID{})
	if *mode == "smoke" {
		result, err := cli.RunSmoke(*data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "smoke failed:", err)
			os.Exit(1)
		}
		fmt.Printf("completed=%d status=%s\n", result.Completed, result.Status)
		return
	}
	if len(flag.Args()) > 0 {
		cmd, err := cli.ParseCommand(flag.Args())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		v, err := cli.Execute(cmd, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(v)
		return
	}
	if err := http.ListenAndServe(*addr, (&httpapi.Server{S: s}).Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
func structRollback() (p domain.RollbackPolicy) { p.MaxFailures = 1; p.Auto = true; return }
