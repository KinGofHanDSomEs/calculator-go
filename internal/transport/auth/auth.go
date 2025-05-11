package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	errs "github.com/kingofhandsomes/calculator-go/internal/errs/auth"
	models "github.com/kingofhandsomes/calculator-go/internal/models/auth"
)

type Auth struct {
	secret   string
	tokenTTL time.Duration
	db       *sql.DB
}

func New(secret string, tokenTTL time.Duration, db *sql.DB) *Auth {
	return &Auth{
		secret:   secret,
		tokenTTL: tokenTTL,
		db:       db,
	}
}

func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	const op = "auth.Register"

	var rreq models.RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&rreq); err != nil {
		log.Printf("%s: %s\n", op, errs.ErrRequestJSON)
		http.Error(w, errs.ErrRequestJSON.Error(), http.StatusUnprocessableEntity)
		return
	}

	login, pass := rreq.Login, rreq.Password

	if err := a.isEmptyLoginPassword(login, pass); err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err := addUser(a.db, login, pass); err != nil {
		if errors.Is(err, errs.ErrRedundantRecording) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("%s: '%s, %s' was successfully registered\n", op, login, pass)
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	const op = "auth.Login"

	var lreq models.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&lreq); err != nil {
		log.Printf("%s: %s\n", op, errs.ErrRequestJSON)
		http.Error(w, errs.ErrRequestJSON.Error(), http.StatusUnprocessableEntity)
		return
	}

	login, pass := lreq.Login, lreq.Password

	if err := a.isEmptyLoginPassword(login, pass); err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	row := a.db.QueryRow(`SELECT login FROM users WHERE login = $1 AND password = $2`, login, pass)

	err := row.Scan(&login)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("%s: %s\n", op, errs.ErrUserNotExist)
			http.Error(w, errs.ErrUserNotExist.Error(), http.StatusUnprocessableEntity)
			return
		}
		log.Printf("%s: error while retrieving the user from the database, error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	token, err := CreateJWTToken(a.tokenTTL, a.secret, login, pass)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(models.LoginResponse{Token: token}); err != nil {
		log.Printf("%s: %s\n", op, errs.ErrServer)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("%s: token for '%s, %s' has been received, token: %s\n", op, login, pass, token)
}

func (a *Auth) isEmptyLoginPassword(login, password string) error {
	if login == "" {
		return errs.ErrRegisterLogin
	}
	if password == "" {
		return errs.ErrRegisterPassword
	}
	return nil
}

func CreateJWTToken(tokenTTL time.Duration, secret, login, password string) (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"login":    login,
		"password": password,
		"nbf":      now.Unix(),
		"exp":      now.Add(tokenTTL).Unix(),
		"iat":      now.Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errs.ErrServer
	}
	return tokenString, nil
}

func addUser(db *sql.DB, login, password string) error {
	const op = "auth.Register.addUser"

	res, err := db.Exec("INSERT INTO users (login, password, count_expressions) VALUES ($1, $2, $3)", login, password, 0)
	if err != nil {
		log.Printf("%s: error inserting a user into the users table, error: %s\n", op, err)
		return errs.ErrRedundantRecording
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("%s: error returning lines from the request, error: %s\n", op, err)
		return errs.ErrServer
	}

	if rowsAffected == 0 {
		log.Printf("%s: %e\n", op, errs.ErrRedundantRecording)
		return errs.ErrRedundantRecording
	}
	return nil
}
