package app_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	authModels "github.com/kingofhandsomes/calculator-go/internal/models/auth"
	orchModels "github.com/kingofhandsomes/calculator-go/internal/models/orchestrator"
	"github.com/kingofhandsomes/calculator-go/internal/transport/agent"
	"github.com/kingofhandsomes/calculator-go/internal/transport/auth"
	"github.com/kingofhandsomes/calculator-go/internal/transport/orchestrator"
	task "github.com/kingofhandsomes/calculator-go/proto"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
)

func TestApp(t *testing.T) {
	os.Remove("./storage.db")
	db, err := sql.Open("sqlite3", "./storage.db")
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("%s", err)
	}

	if _, err := db.Exec("CREATE TABLE users (login TEXT PRIMARY KEY NOT NULL, password TEXT NOT NULL, count_expressions INTEGER NOT NULL)"); err != nil {
		t.Fatalf("error creating table users, error: %s", err)
	}

	if _, err := db.Exec("CREATE TABLE expressions (login TEXT NOT NULL, id_expression INTEGER NOT NULL, expression TEXT NOT NULL, stat TEXT NOT NULL, result REAL NULL, FOREIGN KEY (login) REFERENCES users(login))"); err != nil {
		t.Fatalf("error creating table expressions, error: %s", err)
	}

	if _, err := db.Exec("CREATE TABLE tasks (login TEXT NOT NULL, id_expression INTEGER NOT NULL, id_task INTEGER NOT NULL, arg1 REAL NOT NULL, arg2 REAL NOT NULL, operation STRING NOT NULL, stat STRING NOT NULL, operation_time INTEGER NULL, result REAL NULL)"); err != nil {
		t.Fatalf("error creating table tasks, error: %s", err)
	}

	secret := "aspfdjspgashrgoasrnvpuasrighbousrb"
	ttl := time.Duration(time.Hour)
	grpc_port := 44044
	duration := time.Duration(time.Millisecond)

	a := auth.New(secret, ttl, db)

	testRegisterCases := []struct {
		name               string
		login              string
		password           string
		expectedStatusCode int
	}{
		{
			name:               "register: roman, qwerty",
			login:              "roman",
			password:           "qwerty",
			expectedStatusCode: 200,
		},
		{
			name:               "register: roman1, qwerty1",
			login:              "roman1",
			password:           "qwerty1",
			expectedStatusCode: 200,
		},
		{
			name:               "register: invalid login",
			login:              "",
			password:           "123",
			expectedStatusCode: 422,
		},
	}

	for _, ts := range testRegisterCases {
		t.Run(ts.name, func(t *testing.T) {
			req, _ := json.Marshal(authModels.RegisterRequest{Login: ts.login, Password: ts.password})

			r := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(req))
			w := httptest.NewRecorder()

			a.Register(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}
		})
	}

	testLoginCases := []struct {
		name               string
		login              string
		password           string
		expectedStatusCode int
	}{
		{
			name:               "login: roman, qwerty",
			login:              "roman",
			password:           "qwerty",
			expectedStatusCode: 200,
		},
		{
			name:               "login: roman1, qwerty1",
			login:              "roman1",
			password:           "qwerty1",
			expectedStatusCode: 200,
		},
		{
			name:               "register: invalid login",
			login:              "",
			password:           "123",
			expectedStatusCode: 422,
		},
	}

	for _, ts := range testLoginCases {
		t.Run(ts.name, func(t *testing.T) {
			req, _ := json.Marshal(authModels.LoginRequest{Login: ts.login, Password: ts.password})

			r := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(req))
			w := httptest.NewRecorder()

			a.Login(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}

			if ts.expectedStatusCode == 200 {
				var resp authModels.LoginResponse
				if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
					t.Fatalf("invalid json decode, error: %s", err)
				}

				token, err := auth.CreateJWTToken(ttl, secret, ts.login, ts.password)
				if err != nil {
					t.Fatalf("error creating jwt token, error: %s", err)
				}

				if token != resp.Token {
					t.Fatalf("invalid jwt token, got: %s, want: %s", resp.Token, token)
				}
			}
		})
	}

	o := orchestrator.New(secret, db)

	testCalculateCases := []struct {
		name, login, password, expression string
		ttl                               time.Duration
		expectedStatusCode                int
	}{
		{
			name:               "calculate: add expression to roman",
			login:              "roman",
			password:           "qwerty",
			expression:         "1+2-3*4/5",
			ttl:                ttl,
			expectedStatusCode: 201,
		},
		{
			name:               "calculate: add expression to roman1",
			login:              "roman1",
			password:           "qwerty1",
			expression:         "1+4-6*5+(-3)",
			ttl:                ttl,
			expectedStatusCode: 201,
		},
		{
			name:               "calculate: invalid expression",
			login:              "roman",
			password:           "qwerty",
			expression:         "1+3+4-*5",
			ttl:                ttl,
			expectedStatusCode: 422,
		},
		{
			name:               "calculate: expired jwt token",
			login:              "roman",
			password:           "qwerty",
			expression:         "1+2-3",
			ttl:                0,
			expectedStatusCode: 422,
		},
	}

	for _, ts := range testCalculateCases {
		t.Run(ts.name, func(t *testing.T) {
			token, err := auth.CreateJWTToken(ts.ttl, secret, ts.login, ts.password)
			if err != nil {
				t.Fatalf("error creating jwt token, error: %s", err)
			}

			req, _ := json.Marshal(orchModels.CalculateRequest{Expression: ts.expression})

			if ts.name == "calculate: invalid json" {
				req = nil
			}

			r := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(req))
			r.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()

			o.Calculate(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}
		})
	}

	go func() {
		l, _ := net.Listen("tcp", fmt.Sprintf("localhost:%d", grpc_port))
		grpcServer := grpc.NewServer()
		task.RegisterTaskServiceServer(grpcServer, o)
		grpcServer.Serve(l)
	}()

	agnt := agent.New(db, grpc_port, duration, duration, duration, duration, 3)
	go agnt.MustRun()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			k := 0
			rows, _ := db.Query("SELECT expression FROM expressions WHERE stat = 'calculated'")
			for rows.Next() {
				var expr string
				if rows.Scan(&expr) == nil {
					k++
				}
			}
			if k == 2 {
				break
			}
		}
	}()
	wg.Wait()

	testExpressionsCases := []struct {
		name, login, password string
		ttl                   time.Duration
		expectedStatusCode    int
	}{
		{
			name:               "expressions: get exprs to roman, qwerty",
			login:              "roman",
			password:           "qwerty",
			ttl:                ttl,
			expectedStatusCode: 200,
		},
		{
			name:               "expressions: get exprs to roman1, qwerty1",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                ttl,
			expectedStatusCode: 200,
		},
		{
			name:               "expressions: expired jwt token",
			login:              "roman",
			password:           "qwerty",
			ttl:                0,
			expectedStatusCode: 422,
		},
		{
			name:               "expressions: invalid login",
			login:              "roman2",
			password:           "qwerty",
			ttl:                0,
			expectedStatusCode: 422,
		},
	}

	for _, ts := range testExpressionsCases {
		t.Run(ts.name, func(t *testing.T) {
			token, err := auth.CreateJWTToken(ts.ttl, secret, ts.login, ts.password)
			if err != nil {
				t.Fatalf("error creating jwt token, error: %s", err)
			}

			r := httptest.NewRequest(http.MethodGet, "/api/v1/expressions", nil)
			r.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()

			o.Expressions(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}
		})
	}
}
