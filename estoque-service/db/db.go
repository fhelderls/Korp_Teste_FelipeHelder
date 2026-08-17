package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//Connect abre o pool de conexões com o banco de dados PostgreSQL e confirma se o banco está respondendo de verdade

func Connect(connString string) (*sql.DB, error) {
	database, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir conexão com o banco de dados: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("falha ao pingar o banco de dados: %w", err)
	}
	return database, nil

}

//InitSchema cria a tabela de produtos no banco de dados, caso ela não exista

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS produtos (
			codigo VARCHAR(50) PRIMARY KEY,
			descricao TEXT NOT NULL,
			saldo INTEGER NOT NULL DEFAULT 0,
			preco NUMERIC(10,2) NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// garante a coluna preco em bancos que ja existiam antes dela ser adicionada
	_, err = db.Exec(`ALTER TABLE produtos ADD COLUMN IF NOT EXISTS preco NUMERIC(10,2) NOT NULL DEFAULT 0`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS reservas (
			chave VARCHAR(100) PRIMARY KEY,
			status VARCHAR(20) NOT NULL DEFAULT 'pendente',
			itens TEXT NOT NULL,
			criado_em TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	return err
}
