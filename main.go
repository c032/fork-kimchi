package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var configPath = "/etc/kimchi/config"

func main() {
	flag.StringVar(&configPath, "config", configPath, "configuration file")
	flag.Parse()

	if err := bumpOpenedFileLimit(); err != nil {
		log.Printf("failed to bump max number of opened files: %v", err)
	}

	srv := NewServer()
	if err := loadConfig(srv, configPath); err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}

	<-sigCh
	log.Print("stopping server")
	srv.Stop()
}

func bumpOpenedFileLimit() error {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return fmt.Errorf("failed to get RLIMIT_NOFILE: %v", err)
	}
	rlimit.Cur = rlimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return fmt.Errorf("failed to set RLIMIT_NOFILE: %v", err)
	}
	return nil
}
