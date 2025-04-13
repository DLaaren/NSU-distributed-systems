package database

import (
	"database/sql"
	"lab2/shared"
	"lab2/worker"
)

func CreateTableForWorkers(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS workers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL,
			status TEXT NOT NULL,
			last_hb TIMESTAMP NOT NULL
		)
	`)

	return err
}

func AddWorker(db *sql.DB, worker *worker.Worker) (shared.WorkerId, error) {
	var id shared.WorkerId

	err := db.QueryRow(
		`INSERT INTO workers (address, status, last_hb)
		VALUES ($1, $2, $3)
		RETURNING id`,
		worker.Address, worker.Status, worker.LastHB).
		Scan(&id)

	return id, err
}

func GetWorkerById(db *sql.DB, id shared.WorkerId) (*worker.Worker, error) {
	var worker worker.Worker

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

func GetWorkerByAddress(db *sql.DB, address string) (*worker.Worker, error) {
	var worker worker.Worker

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

func GetAllWorkers(db *sql.DB) ([]*worker.Worker, error) {
	rows, err := db.Query(`
		SELECT *
		FROM workers
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}

	var workers []*worker.Worker
	for rows.Next() {
		var worker worker.Worker
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

func UpdateWorker(db *sql.DB, worker *worker.Worker) error {
	err := db.QueryRow(
		`UPDATE workers
		SET
			address = $1, 
			status = $2, 
			last_hb = $3
		WHERE id = $4`,
		worker.Address, worker.Status, worker.LastHB, worker.Id).
		Err()

	return err
}

func UpdateWorkerStatus(db *sql.DB, worker *worker.Worker) error {
	err := db.QueryRow(
		`UPDATE workers
		SET
			status = $1, 
		WHERE id = $2`,
		shared.DEAD, worker.Id).
		Err()

	return err
}

func DeleteWorker(db *sql.DB, worker *worker.Worker) error {
	err := db.QueryRow(
		`DELETE FROM workers
		WHERE id = $1`,
		worker.Id).
		Err()

	return err
}
