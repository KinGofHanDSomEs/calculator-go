package app

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kingofhandsomes/calculator-go/internal/transport/auth"
	"github.com/kingofhandsomes/calculator-go/internal/transport/orchestrator"
	task "github.com/kingofhandsomes/calculator-go/proto"
	"google.golang.org/grpc"
)

type App struct {
	auth      auth.Auth
	orch      orchestrator.Orchestrator
	port      string
	grpc_port string
}

func New(auth auth.Auth, orch orchestrator.Orchestrator, port int, grpc_port int) *App {
	return &App{
		auth:      auth,
		orch:      orch,
		port:      fmt.Sprint(port),
		grpc_port: fmt.Sprint(grpc_port),
	}
}

func (a *App) MustRunGRPC() {
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", a.grpc_port))
	if err != nil {
		panic("grpc invalid tcp")
	}
	grpcServer := grpc.NewServer()
	task.RegisterTaskServiceServer(grpcServer, &a.orch)
	if err := grpcServer.Serve(l); err != nil {
		panic("grpc startup error")
	}
}

func (a *App) MustRunAPI() {
	r := mux.NewRouter()

	r.HandleFunc("/api/v1/register", a.auth.Register).Methods("POST")
	r.HandleFunc("/api/v1/login", a.auth.Login).Methods("POST")

	r.HandleFunc("/api/v1/calculate", a.orch.Calculate).Methods("POST")
	r.HandleFunc("/api/v1/expressions", a.orch.Expressions).Methods("GET")
	r.HandleFunc("/api/v1/expressions/{id}", a.orch.Expression).Methods("GET")

	if err := http.ListenAndServe(":"+a.port, r); err != nil {
		panic("service startup error")
	}
}
