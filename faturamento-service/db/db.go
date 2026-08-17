package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connect abre pool de conexoes com banco de dados PostgreSQL e confirma
// se o banco esta respondendo
func Connect(connString string) (*sql.DB, error) {
	database, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir conexao com o Banco de Dados: %w", err)

	}
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("falha ao pingar o Banco de Dados: %w", err)

	}
	return database, nil
}

//InitSchema cria a tabela de notas fiscais no Banco de Dados, caso ela nao exista

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS notas (
		chave VARCHAR(100) PRIMARY KEY,
		cliente TEXT NOT NULL,
		itens TEXT NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pendente',
		criado_em TIMESTAMP NOT NULL DEFAULT NOW()
		)
		`)

	return err
}
