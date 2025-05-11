package storage

import "database/sql"

func MustOpenDataBase(driver, storagePath string) *sql.DB {
	db, err := sql.Open(driver, storagePath)
	if err != nil {
		panic("error opening the database")
	}
	if err := db.Ping(); err != nil {
		panic("ping error with the database")
	}
	return db
}
