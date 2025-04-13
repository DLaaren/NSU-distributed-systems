package database

import (
	"database/sql"
)

func Initdb(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = CreateTableForUserRequests(db)
	if err != nil {
		return nil, err
	}
	err = CreateTableForWorkers(db)
	if err != nil {
		return nil, err
	}
	err = CreateTableForTasks(db)
	if err != nil {
		return nil, err
	}

	return db, err
}
