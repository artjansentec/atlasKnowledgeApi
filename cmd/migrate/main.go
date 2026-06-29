package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	migrationsPath, err := migrationsDir()
	if err != nil {
		log.Fatal(err)
	}

	source := "file://" + filepath.ToSlash(migrationsPath)
	m, err := migrate.New(source, databaseURL)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}
	defer m.Close()

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("nenhuma migration pendente")
				break
			}
			if isDirty(err) {
				log.Println("banco em estado dirty — tentando reparar...")
				version, dirty, verr := m.Version()
				if verr == nil && dirty {
					if ferr := m.Force(int(version)); ferr != nil {
						log.Fatalf("migrate repair: %v", ferr)
					}
					fmt.Println("estado dirty corrigido — execute migrate up novamente se necessário")
					break
				}
			}
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations aplicadas")
	case "down":
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("última migration revertida")
	case "repair":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		if !dirty {
			fmt.Printf("banco ok na versão %d\n", version)
			break
		}
		if err := m.Force(int(version)); err != nil {
			log.Fatalf("migrate force: %v", err)
		}
		fmt.Printf("dirty corrigido na versão %d\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func isDirty(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Dirty database")
}

func migrationsDir() (string, error) {
	if dir := os.Getenv("MIGRATIONS_PATH"); dir != "" {
		return filepath.Abs(dir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "internal", "db", "migrations"), nil
}

func printUsage() {
	fmt.Println("Uso: go run ./cmd/migrate [up|down|repair]")
}
