package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/sxyafiq/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID    snowflake.ID
	Email string
	Name  string
}

func main() {
	fmt.Println("=== Snowflake ID Generator - Database Example ===")

	// Open SQLite database (in-memory for demo)
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Generate ID
	gen, _ := snowflake.New(1)
	userID, _ := gen.GenerateID()

	// Insert user (ID automatically converts to int64 for database)
	user := User{
		ID:    userID,
		Email: "alice@example.com",
		Name:  "Alice Johnson",
	}

	fmt.Printf("Inserting user with ID: %d (%s)\n", user.ID.Int64(), user.ID.Base62())
	_, err = db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)",
		user.ID, user.Email, user.Name)
	if err != nil {
		log.Fatal(err)
	}

	// Query user (ID automatically scans from int64)
	var retrieved User
	err = db.QueryRow("SELECT id, email, name FROM users WHERE email = ?", user.Email).
		Scan(&retrieved.ID, &retrieved.Email, &retrieved.Name)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nRetrieved user:")
	fmt.Printf("  ID:    %d\n", retrieved.ID.Int64())
	fmt.Printf("  Email: %s\n", retrieved.Email)
	fmt.Printf("  Name:  %s\n", retrieved.Name)
	fmt.Printf("\nID encodings:")
	fmt.Printf("  Base62:  %s\n", retrieved.ID.Base62())
	fmt.Printf("  Base58:  %s\n", retrieved.ID.Base58())
	fmt.Printf("  Hex:     %s\n", retrieved.ID.Hex())
}
