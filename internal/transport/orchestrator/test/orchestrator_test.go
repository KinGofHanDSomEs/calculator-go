package auth_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
	errs "github.com/kingofhandsomes/calculator-go/internal/errs/orchestrator"
	models "github.com/kingofhandsomes/calculator-go/internal/models/orchestrator"
	"github.com/kingofhandsomes/calculator-go/internal/transport/auth"
	"github.com/kingofhandsomes/calculator-go/internal/transport/orchestrator"
	_ "github.com/mattn/go-sqlite3"
)

func TestOrchestrator(t *testing.T) {
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

	if res, err := db.Exec("INSERT INTO users (login, password, count_expressions) VALUES ('roman', 'qwerty', 0)"); err != nil {
		t.Fatalf("error insert user, error: %s", err)
	} else {
		if n, _ := res.RowsAffected(); n == 0 {
			t.Fatalf("error insert user")
		}
	}
	if res, err := db.Exec("INSERT INTO users (login, password, count_expressions) VALUES ('roman1', 'qwerty1', 0)"); err != nil {
		t.Fatalf("error insert user, error: %s", err)
	} else {
		if n, _ := res.RowsAffected(); n == 0 {
			t.Fatalf("error insert user")
		}
	}

	secret := "aspfdjspgashrgoasrnvpuasrighbousrb"

	o := orchestrator.New(secret, db)

	testCalculateCases := []struct {
		name               string
		login              string
		password           string
		ttl                time.Duration
		expression         string
		expectedStatusCode int
		expectedError      bool
		expectedId         int
		expectedMessage    string
	}{
		{
			name:               "calculate: correctly1",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expression:         "1+2-3*4/5",
			expectedStatusCode: 201,
			expectedError:      false,
			expectedId:         1,
			expectedMessage:    "",
		},
		{
			name:               "calculate: correctly2",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expression:         "1+(2-3)*4/5+(-2)",
			expectedStatusCode: 201,
			expectedError:      false,
			expectedId:         2,
			expectedMessage:    "",
		},
		{
			name:               "calculate: correctly3",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                time.Duration(time.Hour),
			expression:         "14+22-31*47/53",
			expectedStatusCode: 201,
			expectedError:      false,
			expectedId:         1,
			expectedMessage:    "",
		},
		{
			name:               "calculate: correctly4",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                time.Duration(time.Hour),
			expression:         "16+(21-33)*45/45+(-21)",
			expectedStatusCode: 201,
			expectedError:      false,
			expectedId:         2,
			expectedMessage:    "",
		},
		{
			name:               "calculate: invalid expression",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expression:         "16++2",
			expectedStatusCode: 422,
			expectedError:      true,
			expectedId:         0,
			expectedMessage:    errs.ErrExpression.Error(),
		},
		{
			name:               "calculate: token expired",
			login:              "roman",
			password:           "qwerty",
			ttl:                0,
			expression:         "16+1+2",
			expectedStatusCode: 422,
			expectedError:      true,
			expectedId:         0,
			expectedMessage:    errs.ErrTokenExpired.Error(),
		},
		{
			name:               "calculate: invalid login",
			login:              "roman2",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expression:         "16+1+2",
			expectedStatusCode: 422,
			expectedError:      true,
			expectedId:         0,
			expectedMessage:    errs.ErrHeaderAuthorization.Error(),
		},
		{
			name:               "calculate: invalid json",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expression:         "",
			expectedStatusCode: 422,
			expectedError:      true,
			expectedId:         0,
			expectedMessage:    errs.ErrRequestJSON.Error(),
		},
	}

	for _, ts := range testCalculateCases {
		t.Run(ts.name, func(t *testing.T) {
			token, err := auth.CreateJWTToken(ts.ttl, secret, ts.login, ts.password)
			if err != nil {
				t.Fatalf("error creating jwt token, error: %s", err)
			}

			req, _ := json.Marshal(models.CalculateRequest{Expression: ts.expression})

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
			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("invalid reading body pf response, error: %s", err)
			}
			if ts.expectedError {
				message := string(data)
				if len(message) != 0 {
					message = message[:len(message)-1]
				}

				if message != ts.expectedMessage {
					t.Errorf("invalid expected error, got: %s, want: %s", string(message), ts.expectedMessage)
				}
			} else {
				var resp models.CalculateResponse
				if err := json.Unmarshal(data, &resp); err != nil {
					t.Fatalf("invalid json decode, error: %s", err)
				}
				if resp.Id != ts.expectedId {
					t.Errorf("invalid expected id, got: %d, want %d", resp.Id, ts.expectedId)
				}
			}
		})
	}

	testExpressionsCases := []struct {
		name               string
		login              string
		password           string
		ttl                time.Duration
		expectedStatusCode int
		expectedError      bool
		expectedMessage    string
	}{
		{
			name:               "expressions: correctly1",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expressions: correctly2",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                time.Duration(time.Hour),
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expressions: token expired",
			login:              "roman",
			password:           "qwerty",
			ttl:                0,
			expectedStatusCode: 422,
			expectedError:      true,
			expectedMessage:    errs.ErrTokenExpired.Error(),
		},
		{
			name:               "expressions: invalid login",
			login:              "roman2",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			expectedStatusCode: 422,
			expectedError:      true,
			expectedMessage:    errs.ErrHeaderAuthorization.Error(),
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
			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("invalid reading body pf response, error: %s", err)
			}
			if ts.expectedError {
				message := string(data)
				if len(message) != 0 {
					message = message[:len(message)-1]
				}

				if message != ts.expectedMessage {
					t.Errorf("invalid expected error, got: %s, want: %s", string(message), ts.expectedMessage)
				}
			}
		})
	}

	testExpressionCases := []struct {
		name               string
		login              string
		password           string
		ttl                time.Duration
		id                 int
		expectedStatusCode int
		expectedError      bool
		expectedMessage    string
	}{
		{
			name:               "expression: correctly1",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			id:                 1,
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expression: correctly2",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			id:                 2,
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expression: correctly3",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                time.Duration(time.Hour),
			id:                 1,
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expression: correctly4",
			login:              "roman1",
			password:           "qwerty1",
			ttl:                time.Duration(time.Hour),
			id:                 2,
			expectedStatusCode: 200,
			expectedError:      false,
			expectedMessage:    "",
		},
		{
			name:               "expression: token expired",
			login:              "roman",
			password:           "qwerty",
			ttl:                0,
			id:                 0,
			expectedStatusCode: 422,
			expectedError:      true,
			expectedMessage:    errs.ErrTokenExpired.Error(),
		},
		{
			name:               "expression: invalid login",
			login:              "roman2",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			id:                 0,
			expectedStatusCode: 422,
			expectedError:      true,
			expectedMessage:    errs.ErrHeaderAuthorization.Error(),
		},
		{
			name:               "expression: invalid id of expression",
			login:              "roman",
			password:           "qwerty",
			ttl:                time.Duration(time.Hour),
			id:                 3,
			expectedStatusCode: 404,
			expectedError:      true,
			expectedMessage:    errs.ErrExpressionId.Error(),
		},
	}

	for _, ts := range testExpressionCases {
		t.Run(ts.name, func(t *testing.T) {
			token, err := auth.CreateJWTToken(ts.ttl, secret, ts.login, ts.password)
			if err != nil {
				t.Fatalf("error creating jwt token, error: %s", err)
			}

			r := httptest.NewRequest(http.MethodGet, "/api/v1/expressions/"+fmt.Sprint(ts.id), nil)

			r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprint(ts.id)})

			r.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()

			o.Expression(w, r)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != ts.expectedStatusCode {
				t.Errorf("invalid status code, got: %d, want: %d", res.StatusCode, ts.expectedStatusCode)
			}
			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("invalid reading body pf response, error: %s", err)
			}
			if ts.expectedError {
				message := string(data)
				if len(message) != 0 {
					message = message[:len(message)-1]
				}

				if message != ts.expectedMessage {
					t.Errorf("invalid expected error, got: %s, want: %s", string(message), ts.expectedMessage)
				}
			}
		})
	}
}
