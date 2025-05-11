package auth_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	errs "github.com/kingofhandsomes/calculator-go/internal/errs/auth"
	models "github.com/kingofhandsomes/calculator-go/internal/models/auth"
	"github.com/kingofhandsomes/calculator-go/internal/transport/auth"
	_ "github.com/mattn/go-sqlite3"
)

func TestAuth(t *testing.T) {
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
	secret := "aspfdjspgashrgoasrnvpuasrighbousrb"
	ttl := time.Duration(time.Hour)

	a := auth.New(secret, ttl, db)

	testRegisterCases := []struct {
		name               string
		login              string
		password           string
		expectedStatusCode int
		expectedMessage    string
	}{
		{
			name:               "register: simple",
			login:              "roman",
			password:           "qwerty",
			expectedStatusCode: 200,
			expectedMessage:    "",
		},
		{
			name:               "register: repeated login",
			login:              "roman",
			password:           "hello",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRedundantRecording.Error(),
		},
		{
			name:               "register: empty login",
			login:              "",
			password:           "hello",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRegisterLogin.Error(),
		},
		{
			name:               "register: empty password",
			login:              "roman",
			password:           "",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRegisterPassword.Error(),
		},
		{
			name:               "register: invalid json",
			login:              "",
			password:           "",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRequestJSON.Error(),
		},
	}
	for _, ts := range testRegisterCases {
		t.Run(ts.name, func(t *testing.T) {
			req, _ := json.Marshal(models.RegisterRequest{Login: ts.login, Password: ts.password})

			if ts.name == "register: invalid json" {
				req = nil
			}

			r := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(req))
			w := httptest.NewRecorder()

			a.Register(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}

			message, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("invalid reading body pf response, error: %s", err)
			}

			messageString := string(message)
			if len(messageString) != 0 {
				messageString = messageString[:len(messageString)-1]
			}

			if messageString != ts.expectedMessage {
				t.Errorf("invalid exprected error, got: %s, want: %s", string(message), ts.expectedMessage)
			}
		})
	}

	testLoginCases := []struct {
		name               string
		login              string
		password           string
		expectedStatusCode int
		expectedMessage    string
	}{
		{
			name:               "login: simple",
			login:              "roman",
			password:           "qwerty",
			expectedStatusCode: 200,
			expectedMessage:    "",
		},
		{
			name:               "login: invalid login",
			login:              "rrrr",
			password:           "hello",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrUserNotExist.Error(),
		},
		{
			name:               "login: invalid password",
			login:              "roman",
			password:           "hello",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrUserNotExist.Error(),
		},
		{
			name:               "login: empty login",
			login:              "",
			password:           "hello",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRegisterLogin.Error(),
		},
		{
			name:               "login: empty password",
			login:              "roman",
			password:           "",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRegisterPassword.Error(),
		},
		{
			name:               "login: invalid json",
			login:              "",
			password:           "",
			expectedStatusCode: 422,
			expectedMessage:    errs.ErrRequestJSON.Error(),
		},
	}
	for _, ts := range testLoginCases {
		t.Run(ts.name, func(t *testing.T) {
			req, _ := json.Marshal(models.RegisterRequest{Login: ts.login, Password: ts.password})

			if ts.name == "login: invalid json" {
				req = nil
			}

			r := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(req))
			w := httptest.NewRecorder()

			a.Login(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}

			if ts.expectedMessage == "" {
				t.Skip()
			}
			message, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("invalid reading body pf response, error: %s", err)
			}

			messageString := string(message)
			if len(messageString) != 0 {
				messageString = messageString[:len(messageString)-1]
			}

			if messageString != ts.expectedMessage {
				t.Errorf("invalid exprected error, got: %s, want: %s", string(message), ts.expectedMessage)
			}
		})
	}
}
