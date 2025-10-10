package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/sxyafiq/snowflake"
)

type UserResponse struct {
	ID    snowflake.ID `json:"id"`
	Email string       `json:"email"`
	Name  string       `json:"name"`
}

func main() {
	fmt.Println("=== Snowflake ID Generator - JSON Example ===")

	// Generate ID
	gen, _ := snowflake.New(1)
	userID, _ := gen.GenerateID()

	// Create response object
	response := UserResponse{
		ID:    userID,
		Email: "bob@example.com",
		Name:  "Bob Smith",
	}

	// Marshal to JSON (ID is encoded as string to avoid JavaScript precision loss)
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("JSON output (ID as string for JavaScript safety):")
	fmt.Println(string(jsonData))
	fmt.Println()

	// Unmarshal from JSON
	var decoded UserResponse
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Decoded from JSON:")
	fmt.Printf("  ID:    %d\n", decoded.ID.Int64())
	fmt.Printf("  Email: %s\n", decoded.Email)
	fmt.Printf("  Name:  %s\n", decoded.Name)
	fmt.Println()

	// Demonstrate custom format marshaling
	formatted := snowflake.IDWithFormat{
		ID:     userID,
		Format: "base62",
	}

	formattedJSON, _ := json.Marshal(formatted)
	fmt.Println("Custom format (Base62) JSON:")
	fmt.Println(string(formattedJSON))
}
