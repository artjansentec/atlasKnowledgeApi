package main

import (
	"fmt"
	"log"
	"os"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL não definido — copie .env.example para .env")
	}

	switch os.Args[1] {
	case "up":
		if err := db.MigrateUp(databaseURL); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations aplicadas")
	case "down":
		if err := db.MigrateDown(databaseURL); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("última migration revertida")
	case "repair":
		version, repaired, err := db.MigrateRepair(databaseURL)
		if err != nil {
			log.Fatalf("migrate repair: %v", err)
		}
		if repaired {
			fmt.Printf("dirty corrigido na versão %d\n", version)
		} else {
			fmt.Printf("banco ok na versão %d\n", version)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Uso: go run ./cmd/migrate [up|down|repair]")
}
