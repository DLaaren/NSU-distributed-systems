package database

import (
	"database/sql"

	"lab2/shared"
	"lab2/task"
)

func CreateTableForTasks(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			input_range TEXT NOT NULL,
			max_length INTEGER NOT NULL,
			status TEXT NOT NULL,
			result JSON,
			user_request_id INTEGER NOT NULL,
			worker_id INTEGER,
			FOREIGN KEY (user_request_id) REFERENCES user_requests(id) ON DELETE CASCADE,
			FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE SET NULL
		)
	`)

	return err
}

func AddTask(db *sql.DB, task *task.Task) error {
	var id shared.TaskId

	err := db.QueryRow(
		`INSERT INTO tasks (input_range, max_length, status, user_request_id, worker_id)
		"VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		task.InputRange, task.MaxLength, task.IN_PROGRESS, task.RequestId, task.WorkerId).
		Scan(&id)

	return err
}

func GetTasksByRequestId(db *sql.DB, requestId shared.UserRequestId) ([]task.Task, error) {
	rows, err := db.Query(`
		SELECT status, result
		FROM tasks
		WHERE request_id = $1
	`, requestId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tasks []task.Task
	for rows.Next() {
		var task task.Task
		err = rows.Scan(
			&task.Id,
			&task.InputRange,
			&task.MaxLength,
			&task.Status,
			&task.Result,
			&task.RequestId,
			&task.WorkerId)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

func GetTaskResultsByRequestId(db *sql.DB, requestId shared.UserRequestId) ([]task.TaskResultResponse, error) {
	rows, err := db.Query(`
		SELECT status, result
		FROM tasks
		WHERE request_id = $1
	`, requestId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []task.TaskResultResponse
	for rows.Next() {
		var status task.TaskStatus
		var result []string

		err = rows.Scan(&status, &result)
		if err != nil {
			return nil, err
		}
		results = append(results, task.TaskResultResponse{Status: status, Result: result})
	}

	return results, rows.Err()
}

func UpdateTaskStatusAndResult(db *sql.DB, task *task.Task) error {
	err := db.QueryRow(
		`UPDATE tasks
		SET
			status = $1, 
			result = $2, 
		WHERE id = $3`,
		task.Status, task.Result, task.Id).
		Err()

	return err
}

func UpdateTaskStatusAndResultByRequestId(db *sql.DB, requestId shared.UserRequestId, status task.TaskStatus, result []string) error {
	err := db.QueryRow(
		`UPDATE tasks
		SET
			status = $1, 
			result = $2, 
		WHERE user_request_id = $3`,
		status, result, requestId).
		Err()

	return err
}

func CountCompleteTasks(db *sql.DB, requestId shared.UserRequestId) (bool, error) {
	var total, completed int
	err := db.QueryRow(
		`SELECT 
            COUNT(*) AS total,
            SUM(CASE WHEN status = 'DONE_SUCCESS' OR status = 'DONE_FAILURE' 
			THEN 1 ELSE 0 END) AS completed
        FROM tasks
        WHERE request_id = ?`,
		requestId).
		Scan(&total, &completed)

	var allCompleted bool
	if total-completed > 0 {
		allCompleted = false
	} else if total-completed == 0 {
		allCompleted = true
	}
	return allCompleted, err
}
