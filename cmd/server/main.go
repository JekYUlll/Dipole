package main

import (
	"log"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/server"
	"github.com/JekYUlll/Dipole/internal/store"
)

func main() {
	config.MustLoad()

	appCfg := config.AppConfig()

	if err := store.InitMySQL(); err != nil {
		log.Fatalf("mysql init failed: %v", err)
	}

	if err := store.InitRedis(); err != nil {
		log.Fatalf("redis init failed: %v", err)
	}

	srv := server.New()

	log.Printf("%s starting in %s on %s", appCfg.Name, appCfg.Env, config.Addr())

	if err := srv.Run(config.Addr()); err != nil {
		log.Fatalf("server run failed: %v", err)
	}
}
