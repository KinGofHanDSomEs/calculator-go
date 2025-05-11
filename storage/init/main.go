package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	os.Remove("./storage/storage.db")

	db, err := sql.Open("sqlite3", "storage/storage.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	createUsersTable := `  
    CREATE TABLE users (
		login TEXT PRIMARY KEY NOT NULL,
		password TEXT NOT NULL,
		count_expressions INTEGER NOT NULL
	);`
	if _, err := db.Exec(createUsersTable); err != nil {
		log.Fatalf("error when creating the users table: %v", err)
	}

	createExpressionsTable := ` 
    CREATE TABLE expressions (
		login TEXT NOT NULL,
		id_expression INTEGER NOT NULL,
		expression TEXT NOT NULL,
		stat TEXT NOT NULL,
		result REAL NULL,
		FOREIGN KEY (login) REFERENCES users(login)
	);`
	if _, err := db.Exec(createExpressionsTable); err != nil {
		log.Fatalf("error when creating the expressions table: %v", err)
	}
	createTasksTable := ` 
    CREATE TABLE tasks (
		login TEXT NOT NULL,
		id_expression INTEGER NOT NULL,
		id_task INTEGER NOT NULL,
		arg1 REAL NOT NULL,
		arg2 REAL NOT NULL,
		operation STRING NOT NULL,
		stat STRING NOT NULL,
		operation_time INTEGER NULL,
		result REAL NULL
	);`
	if _, err := db.Exec(createTasksTable); err != nil {
		log.Fatalf("error when creating the tasks table: %v", err)
	}

	log.Println("the database and tables have been successfully recreated")
}
