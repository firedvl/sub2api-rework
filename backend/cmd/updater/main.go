package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Wei-Shaw/sub2api/internal/updater"
)

func main() {
	configPath := flag.String("config", "/etc/sub2api-rework/updater.json", "Path to the root-owned updater policy")
	showVersion := flag.Bool("version", false, "Show updater version")
	flag.Parse()
	if *showVersion {
		fmt.Println(updater.Version)
		return
	}
	policy, err := updater.LoadPolicy(*configPath)
	if err != nil {
		log.Fatalf("load updater policy: %v", err)
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
