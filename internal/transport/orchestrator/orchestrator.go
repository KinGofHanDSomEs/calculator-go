package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	errs "github.com/kingofhandsomes/calculator-go/internal/errs/orchestrator"
	models "github.com/kingofhandsomes/calculator-go/internal/models/orchestrator"
	task "github.com/kingofhandsomes/calculator-go/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Orchestrator struct {
	secret string
	db     *sql.DB
	task.TaskServiceServer
}

func New(secret string, db *sql.DB) *Orchestrator {
	return &Orchestrator{
		secret: secret,
		db:     db,
	}
}

// /api/v1/calculate
func (o *Orchestrator) Calculate(w http.ResponseWriter, r *http.Request) {
	const op = "orchestrator.Calculate"
	var creq models.CalculateRequest

	if err := json.NewDecoder(r.Body).Decode(&creq); err != nil {
		log.Printf("%s: %s\n", op, errs.ErrRequestJSON)
		http.Error(w, errs.ErrRequestJSON.Error(), http.StatusUnprocessableEntity)
		return
	}

	login, err := checkJWT(r.Header.Get("Authorization"), o.secret)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		if errors.Is(err, jwt.ErrTokenExpired) {
			http.Error(w, errs.ErrTokenExpired.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	tx, _ := o.db.Begin()
	defer tx.Rollback()

	var lg string
	result := tx.QueryRow("SELECT login FROM users WHERE login = $1", login)
	if result.Scan(&lg) != nil {
		log.Printf("%s: unregistered user\n", op)
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	expr := strings.ReplaceAll(creq.Expression, " ", "")
	if !isValidExpression(expr) {
		log.Printf("%s: %s\n", op, errs.ErrExpression)
		http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
		return
	}

	rpn, err := infixToRPN(expr)
	if err != nil {
		log.Printf("%s: error when converting an expression to reverse polish notation\n", op)
		http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
		return
	}

	row := tx.QueryRow("SELECT count_expressions FROM users WHERE login = $1", login)

	var id_expression int

	err = row.Scan(&id_expression)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("%s: '%s' login is not in the users table\n", op, login)
			http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
			return
		}
		log.Printf("%s: error while retrieving the user from the database, error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}
	id_expression++
	isFirstTask := true
	id_task := 1
	var stack []float64
	for _, oper := range rpn {
		if n, err := strconv.ParseFloat(oper, 64); err != nil {
			if len(stack) < 2 {
				log.Printf("%s: task composition error\n", op)
				http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
				return
			}
			arg1, arg2 := stack[len(stack)-2], stack[len(stack)-1]
			var stts string
			stts = "not ready"
			if isFirstTask {
				stts = "ready"
				isFirstTask = false
			}
			_, err := tx.Exec("INSERT INTO tasks (login, id_expression, id_task, arg1, arg2, operation, stat) VALUES ($1, $2, $3, $4, $5, $6, $7)", login, id_expression, id_task, arg1, arg2, oper, stts)
			if err != nil {
				log.Printf("%s: %s\n", op, err)
				http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
				return
			}
			id_task++
			stack = stack[:len(stack)-2]
			var res float64
			switch oper {
			case "+":
				res = arg1 + arg2
			case "-":
				res = arg1 - arg2
			case "*":
				res = arg1 * arg2
			case "/":
				if arg2 == 0 {
					log.Printf("%s: error dividing by zero\n", op)
					http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
					return
				}
				res = arg1 / arg2
			default:
				log.Printf("%s: invalid symbol: %s, in expression\n", op, oper)
				http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
				return
			}
			stack = append(stack, res)
			continue
		} else {
			stack = append(stack, n)
		}
	}
	if len(stack) != 1 {
		log.Printf("%s: task composition error\n", op)
		http.Error(w, errs.ErrExpression.Error(), http.StatusUnprocessableEntity)
		return
	}

	res, err := tx.Exec("INSERT INTO expressions (login, id_expression, expression, stat, result) VALUES ($1, $2, $3, $4, $5)", login, id_expression, expr, "not calculated", 0)
	if err != nil {
		log.Printf("%s: error inserting a expression into the expressions table, error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected, err := res.RowsAffected(); err != nil || rowsAffected == 0 {
		log.Printf("%s: error returning lines from the request, error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	res, err = tx.Exec("UPDATE users SET count_expressions = $1 WHERE login = $2", id_expression, login)
	if err != nil {
		log.Printf("%s: error updating the number of expressions for a user with a login: %s, error: %s\n", op, login, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected, err := res.RowsAffected(); err != nil || rowsAffected == 0 {
		log.Printf("%s: error returning lines from the request, error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("%s: transaction capture error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(models.CalculateResponse{Id: id_expression}); err != nil {
		log.Printf("%s: %s\n", op, errs.ErrServer)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("%s: expression %s for the login %s was added\n", op, expr, login)
}

// /api/v1/expressions
func (o *Orchestrator) Expressions(w http.ResponseWriter, r *http.Request) {
	const op = "orchestrator.Expressions"

	login, err := checkJWT(r.Header.Get("Authorization"), o.secret)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		if errors.Is(err, jwt.ErrTokenExpired) {
			http.Error(w, errs.ErrTokenExpired.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	tx, _ := o.db.Begin()
	defer tx.Rollback()

	var lg string
	res := tx.QueryRow("SELECT login FROM users WHERE login = $1", login)
	if res.Scan(&lg) != nil {
		log.Printf("%s: unregistered user\n", op)
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	rows, err := tx.Query("SELECT id_expression, stat, result FROM expressions WHERE login = $1", login)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	var exprs []models.ExpressionResponse

	for rows.Next() {
		var expr models.ExpressionResponse

		err := rows.Scan(&expr.Id, &expr.Status, &expr.Result)
		if err != nil {
			log.Printf("%s: %s\n", op, err)
			http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
			return
		}
		exprs = append(exprs, expr)
	}

	if err = tx.Commit(); err != nil {
		log.Printf("%s: transaction capture error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string][]models.ExpressionResponse{"expressions": exprs}); err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("%s: output of all expressions for the login: %s\n", op, login)
}

func (o *Orchestrator) Expression(w http.ResponseWriter, r *http.Request) {
	const op = "orchestrator.Expression"

	login, err := checkJWT(r.Header.Get("Authorization"), o.secret)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		if errors.Is(err, jwt.ErrTokenExpired) {
			http.Error(w, errs.ErrTokenExpired.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	tx, _ := o.db.Begin()
	defer tx.Rollback()

	var lg string
	res := tx.QueryRow("SELECT login FROM users WHERE login = $1", login)
	if res.Scan(&lg) != nil {
		log.Printf("%s: unregistered user\n", op)
		http.Error(w, errs.ErrHeaderAuthorization.Error(), http.StatusUnprocessableEntity)
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, errs.ErrExpressionId.Error(), http.StatusNotFound)
		return
	}

	row := tx.QueryRow("SELECT id_expression, stat, result FROM expressions WHERE login = $1 AND id_expression = $2", login, id)

	var expr models.ExpressionResponse

	err = row.Scan(&expr.Id, &expr.Status, &expr.Result)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("%s: %s\n", op, errs.ErrExpressionId)
			http.Error(w, errs.ErrExpressionId.Error(), http.StatusNotFound)
			return
		}
		log.Printf("%s: %s\n", op, errs.ErrServer)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("%s: transaction capture error: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]models.ExpressionResponse{"expression": expr}); err != nil {
		log.Printf("%s: %s\n", op, err)
		http.Error(w, errs.ErrServer.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("%s: output of an expression with an id: %d, for the login: %s\n", op, id, login)
}

func (o *Orchestrator) GetTask(context.Context, *task.GetTaskRequest) (*task.GetTaskResponse, error) {
	var resp task.GetTaskResponse
	err := o.db.QueryRow("SELECT login, id_expression, id_task, arg1, arg2, operation FROM tasks WHERE stat = 'ready'").Scan(&resp.Login, &resp.IdExpression, &resp.IdTask, &resp.Arg1, &resp.Arg2, &resp.Operation)
	if err != nil {
		return nil, status.Error(codes.NotFound, "task not found")
	}
	o.db.Exec("UPDATE tasks SET stat = 'in progress' WHERE login = $1 AND id_expression = $2 AND id_task = $3", resp.Login, resp.IdExpression, resp.IdTask)
	return &resp, nil
}

func (o *Orchestrator) PostTask(ctx context.Context, req *task.PostTaskRequest) (*task.PostTaskResponse, error) {
	const op = "orchestrator.PostTask"

	tx, _ := o.db.Begin()
	defer tx.Rollback()

	_, err := tx.Exec("UPDATE tasks SET stat = 'calculated', operation_time = $1, result = $2 WHERE login = $3 AND id_expression = $4 AND id_task = $5", req.OperationTime, req.Result, req.Login, req.IdExpression, req.IdTask)
	if err != nil {
		log.Printf("%s: %s\n", op, err)
		return nil, status.Error(codes.Internal, "server error")
	}

	res, _ := tx.Exec("UPDATE tasks SET stat = 'ready' WHERE login = $1 AND id_expression = $2 AND id_task = $3", req.Login, req.IdExpression, req.IdTask+1)
	if n, _ := res.RowsAffected(); n == 0 {
		log.Printf("%s: expression was calculated, login: %s, id of expression: %d\n", op, req.GetLogin(), req.GetIdExpression())
		tx.Exec("UPDATE expressions SET stat = 'calculated', result = $1 WHERE login = $2 AND id_expression = $3", req.Result, req.Login, req.IdExpression)
	}

	if err = tx.Commit(); err != nil {
		log.Printf("%s: transaction capture error: %s\n", op, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &task.PostTaskResponse{}, nil
}

var precedence = map[rune]int{
	'+': 1,
	'-': 1,
	'*': 2,
	'/': 2,
}

func isOperator(r rune) bool {
	return r == '+' || r == '-' || r == '*' || r == '/'
}

func tokenize(expr string) ([]string, error) {
	var tokens []string
	var number strings.Builder

	expr = strings.ReplaceAll(expr, " ", "")

	for i, ch := range expr {
		if unicode.IsDigit(ch) {
			number.WriteRune(ch)
		} else if ch == '-' {
			if i == 0 || (i > 0 && (expr[i-1] == '(' || isOperator(rune(expr[i-1])))) {
				number.WriteRune(ch)
				continue
			} else {
				if number.Len() > 0 {
					tokens = append(tokens, number.String())
					number.Reset()
				}
				tokens = append(tokens, "-")
			}
		} else if isOperator(rune(ch)) || ch == '(' || ch == ')' {
			if number.Len() > 0 {
				tokens = append(tokens, number.String())
				number.Reset()
			}
			tokens = append(tokens, string(ch))
		} else {
			return nil, errors.New("invalid symbol: " + string(ch))
		}
	}
	if number.Len() > 0 {
		tokens = append(tokens, number.String())
	}

	return tokens, nil
}

func infixToRPN(expr string) ([]string, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return nil, err
	}

	var output []string
	var stack []rune

	for _, token := range tokens {
		if _, err := strconv.Atoi(token); err == nil {
			output = append(output, token)
		} else if len(token) == 1 && isOperator(rune(token[0])) {
			currOp := rune(token[0])
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if isOperator(top) && precedence[top] >= precedence[currOp] {
					output = append(output, string(top))
					stack = stack[:len(stack)-1]
				} else {
					break
				}
			}
			stack = append(stack, currOp)
		} else if token == "(" {
			stack = append(stack, '(')
		} else if token == ")" {
			found := false
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if top == '(' {
					stack = stack[:len(stack)-1]
					found = true
					break
				}
				output = append(output, string(top))
				stack = stack[:len(stack)-1]
			}
			if !found {
				return nil, errors.New("bracket mismatch")
			}
		} else {
			return nil, errors.New("invalid symbol: " + token)
		}
	}

	for len(stack) > 0 {
		top := stack[len(stack)-1]
		if top == '(' || top == ')' {
			return nil, errors.New("bracket mismatch")
		}
		output = append(output, string(top))
		stack = stack[:len(stack)-1]
	}

	return output, nil
}

func checkJWT(jwt_token, secret string) (string, error) {
	if jwt_token == "" {
		return "", errors.New("header 'Authorization' was not found")
	}

	parts := strings.Split(jwt_token, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("incorrect description of header 'Authorization'")
	}

	tokenString := parts[1]

	tokenFromString, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method: " + fmt.Sprint(token.Header["alg"]))
		}

		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := tokenFromString.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("the login key was not found")
	}

	return fmt.Sprint(claims["login"]), nil

}

func isValidExpression(expr string) bool {
	validPattern := `^[-+*/()\d]+$`
	matched, _ := regexp.MatchString(validPattern, expr)
	if !matched {
		return false
	}
	stack := []rune{}
	for _, ch := range expr {
		if ch == '(' {
			stack = append(stack, ch)
		} else if ch == ')' {
			if len(stack) == 0 {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return false
	}

	syntaxPattern := `([\+\-\*/]{2,})|([\+\-\*/][\)])|([$$][$$])|([$$]$)|(^[\+\*/])`
	syntaxRegex := regexp.MustCompile(syntaxPattern)

	return !syntaxRegex.MatchString(expr)
}
