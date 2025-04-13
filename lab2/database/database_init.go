package database

import (
	"database/sql"
)

func Initdb(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return db, err
	}

	CreateTableForUserRequests(db)
	CreateTableForWorkers(db)
	CreateTableForTasks(db)

	return db, err
}
