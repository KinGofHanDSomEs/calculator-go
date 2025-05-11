package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kingofhandsomes/calculator-go/internal/app"
	"github.com/kingofhandsomes/calculator-go/internal/config"
	"github.com/kingofhandsomes/calculator-go/internal/transport/agent"
	"github.com/kingofhandsomes/calculator-go/internal/transport/auth"
	"github.com/kingofhandsomes/calculator-go/internal/transport/orchestrator"
	"github.com/kingofhandsomes/calculator-go/storage"
	_ "github.com/mattn/go-sqlite3"
)

const secret = "aspfdjspgashrgoasrnvpuasrighbousrb"

func main() {
	cfg := config.MustLoad()

	log.Printf("config has been initialized: %v\n", cfg)

	db := storage.MustOpenDataBase("sqlite3", cfg.StoragePath)
	defer db.Close()

	auth := auth.New(secret, cfg.TokenTTL, db)
	orch := orchestrator.New(secret, db)

	application := app.New(*auth, *orch, cfg.Port, cfg.GRPCPort)
	go application.MustRunGRPC()
	go application.MustRunAPI()

	agnt := agent.New(db, cfg.GRPCPort, cfg.TimeAdditon, cfg.TimeSubtraction, cfg.TimeMultiplications, cfg.TimeDivisions, cfg.ComputingPower)
	go agnt.MustRun()

	log.Printf("services are running, port: %d, GRPC port: %d\n", cfg.Port, cfg.GRPCPort)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	sign := <-stop

	log.Printf("service stopped, signal: %v\n", sign)
}
