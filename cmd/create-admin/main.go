package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/atlas/knowledge-api/internal/bootstrap"
	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	email := flag.String("email", "", "e-mail do administrador (obrigatório)")
	password := flag.String("password", "", "senha (obrigatório)")
	name := flag.String("name", "Administrador", "nome exibido")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Println("Uso: go run ./cmd/create-admin -email admin@empresa.com -password SUA_SENHA [-name Nome]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("banco de dados: %v", err)
	}
	defer database.Close()

	result, err := bootstrap.EnsureDefaultAdmin(ctx, database, bootstrap.EnsureAdminOptions{
		Email:    *email,
		Password: *password,
		Name:     *name,
	})
	if err != nil {
		log.Fatalf("criar admin: %v", err)
	}
	if !result.Created {
		log.Fatal("já existem usuários no banco — comando abortado")
	}

	fmt.Printf("admin criado: %s (%s)\n", result.Name, result.Email)
}
