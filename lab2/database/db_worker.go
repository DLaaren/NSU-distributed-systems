package database

import (
	"database/sql"
	"lab2/shared"
	"lab2/worker"
)

func CreateTableForWorkers(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS workers (
			id SERIAL PRIMARY KEY,
			address TEXT NOT NULL,
			status TEXT NOT NULL,
			last_hb TIMESTAMP NOT NULL
		)
	`)

	return err
}

func AddWorker(db *sql.DB, worker *pworker.Worker) (shared.WorkerId, error) {
	var id shared.WorkerId

	err := db.QueryRow(
		`INSERT INTO workers (address, status, last_hb)
		VALUES ($1, $2, $3)
		RETURNING id`,
		worker.Address, worker.Status, worker.LastHB).
		Scan(&id)

	return id, err
}

func GetWorkerById(db *sql.DB, id shared.WorkerId) (*pworker.Worker, error) {
	var worker pworker.Worker

	err := db.QueryRow(
		`SELECT * 
		FROM workers
		WHERE id = $1`,
		id).
		Scan(&worker.Id,
			&worker.Address,
			&worker.Status,
			&worker.LastHB)

	return &worker, err
}

func GetWorkerByAddress(db *sql.DB, address string) (*pworker.Worker, error) {
	var worker pworker.Worker

	err := db.QueryRow(
		`SELECT * 
		FROM workers
		WHERE address = $1`,
		address).
		Scan(&worker.Id,
			&worker.Address,
			&worker.Status,
			&worker.LastHB)

	return &worker, err
}

func GetAllWorkers(db *sql.DB) ([]*pworker.Worker, error) {
	rows, err := db.Query(`
		SELECT *
		FROM workers
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var workers []*pworker.Worker
	for rows.Next() {
		var worker pworker.Worker
		err = rows.Scan(
			&worker.Id,
			&worker.Address,
			&worker.Status,
			&worker.LastHB,
		)
		if err != nil {
			return nil, err
		}

		workers = append(workers, &worker)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return workers, err
}

func GetAllAliveWorkers(db *sql.DB) ([]*pworker.Worker, error) {
	rows, err := db.Query(`
		SELECT *
		FROM workers
		WHERE status = 'ALIVE'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var workers []*pworker.Worker
	for rows.Next() {
		var worker pworker.Worker
		err = rows.Scan(
			&worker.Id,
			&worker.Address,
			&worker.Status,
			&worker.LastHB,
		)
		if err != nil {
			return nil, err
		}

		workers = append(workers, &worker)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return workers, err
}

func UpdateWorker(db *sql.DB, worker *pworker.Worker) (shared.WorkerId, error) {
	var id shared.WorkerId
	err := db.QueryRow(
		`UPDATE workers
		SET
			status = $2, 
			last_hb = $3
		WHERE address = $1
		RETURNING id`,
		worker.Address, worker.Status, worker.LastHB).
		Scan(&id)

	return id, err
}

func UpdateWorkerStatus(db *sql.DB, worker *pworker.Worker) error {
	err := db.QueryRow(
		`UPDATE workers
		SET
			status = $1, 
		WHERE id = $2`,
		pworker.DEAD, worker.Id).
		Err()

	return err
}

func DeleteWorker(db *sql.DB, worker *pworker.Worker) error {
	err := db.QueryRow(
		`DELETE FROM workers
		WHERE id = $1`,
		worker.Id).
		Err()

	return err
}
