package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

// SQL query in raw string literal (backticks)
const createUserTable = `
CREATE TABLE IF NOT EXISTS accounts (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
`

func main() {
	db, _ := sql.Open("postgres", "user=foo password=bar dbname=test")
	
	// Executing a complex SELECT in an interpreted string literal
	rows, _ := db.Query("SELECT id, username FROM accounts WHERE id > 100")
	fmt.Println(rows)

	// HTML template in Go
	const htmlTemplate = `
	<div>
		<h1>User Account</h1>
		<p>Welcome back!</p>
	</div>
	`
	fmt.Println(htmlTemplate)
}
