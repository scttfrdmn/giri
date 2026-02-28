// sql_query verifies that database/sql intercepts work (#102).
//
// Rows.Next returns false so the scan loop body is never entered;
// this is correct conservative behavior.
//
// Expected: 0 violations.
package main

import "database/sql"

func main() {
	// sql.Open returns (*DB, error).
	db, err := sql.Open("sqlite3", ":memory:")
	_ = err

	// Ping.
	_ = db.Ping()

	// Query returns (*Rows, error).
	rows, err2 := db.Query("SELECT 1")
	_ = err2

	// Rows.Next returns false in the interpreter's model.
	for rows.Next() {
		var v int
		_ = rows.Scan(&v)
	}
	_ = rows.Err()
	_ = rows.Close()

	// QueryRow returns *Row.
	row := db.QueryRow("SELECT 1")
	var x int
	_ = row.Scan(&x)

	// Exec returns (Result, error).
	result, err3 := db.Exec("INSERT INTO t VALUES (1)")
	_ = result
	_ = err3

	// Begin returns (*Tx, error).
	tx, err4 := db.Begin()
	_ = err4
	_ = tx.Commit()

	// Close.
	_ = db.Close()
}
