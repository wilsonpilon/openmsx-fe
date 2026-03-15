package main

import (
	"log"
	"os"

	"msxfront/internal/db"
	"msxfront/internal/ui"
)

func main() {
	// Initialize SQLite database
	database, err := db.New("msxfront.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Setup log file
	logFile, err := os.OpenFile("msxfront.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// Launch TUI
	app := ui.NewApp(database)
	if err := app.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
