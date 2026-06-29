package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/service"
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL não definido — copie .env.example para .env")
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("banco de dados: %v", err)
	}
	defer database.Close()

	var count int
	if err := database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		log.Fatalf("consulta users: %v", err)
	}
	if count > 0 {
		log.Fatal("já existem usuários no banco — comando abortado")
	}

	hash, err := service.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash senha: %v", err)
	}

	user := domain.User{
		Email:        *email,
		PasswordHash: hash,
		Name:         *name,
		Role:         domain.RoleAdmin,
		IsActive:     true,
	}

	tx, err := database.Pool.Begin(ctx)
	if err != nil {
		log.Fatalf("transação: %v", err)
	}
	defer tx.Rollback(ctx)

	users := repository.NewUserRepository(database)
	if err := users.Create(ctx, tx, &user); err != nil {
		log.Fatalf("criar admin: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("admin criado: %s (%s)\n", user.Name, user.Email)
}
