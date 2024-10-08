package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var configPath = "/etc/kimchi/config"

func main() {
	flag.StringVar(&configPath, "config", configPath, "configuration file")
	flag.Parse()

	srv := NewServer()
	if err := loadConfig(srv, configPath); err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}

	for sig := range sigCh {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Print("stopping server")
			srv.Stop()
			return
		case syscall.SIGHUP:
			log.Print("reloading config")
			newSrv := NewServer()
			if err := loadConfig(newSrv, configPath); err != nil {
				log.Printf("reload failed: %v", err)
				continue
			}
			if err := newSrv.Replace(srv); err != nil {
				log.Printf("reload failed: %v", err)
				continue
			}
			srv = newSrv
			log.Print("config reloaded")
		}
	}
}
