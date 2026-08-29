package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Wei-Shaw/sub2api/internal/updater"
)

func main() {
	configPath := flag.String("config", "/etc/sub2api-rework/updater.json", "Path to the root-owned updater policy")
	showVersion := flag.Bool("version", false, "Show updater version")
	printSystemdDropIn := flag.Bool("print-systemd-drop-in", false, "Print the deployment-specific systemd sandbox drop-in")
	flag.Parse()
	if *showVersion {
		fmt.Println(updater.Version)
		return
	}
	policy, err := updater.LoadPolicy(*configPath)
	if err != nil {
		log.Fatalf("load updater policy: %v", err)
	}
	if *printSystemdDropIn {
		executablePath, err := os.Executable()
		if err != nil {
			log.Fatalf("resolve updater executable: %v", err)
		}
		resolvedExecutablePath, err := filepath.EvalSymlinks(executablePath)
		if err != nil {
			log.Fatalf("resolve updater executable symlinks: %v", err)
		}
		dropIn, err := updater.SystemdDropIn(policy, *configPath, executablePath, resolvedExecutablePath)
		if err != nil {
			log.Fatalf("render systemd drop-in: %v", err)
		}
		if _, err := os.Stdout.Write(dropIn); err != nil {
			log.Fatalf("write systemd drop-in: %v", err)
		}
		return
	}
	service, err := updater.NewService(policy, nil, nil)
	if err != nil {
		log.Fatalf("initialize updater: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := updater.ServeUnix(ctx, policy, service.Handler()); err != nil {
		log.Fatalf("serve updater: %v", err)
	}
}
