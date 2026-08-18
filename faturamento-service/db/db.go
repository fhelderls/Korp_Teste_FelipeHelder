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
		status VARCHAR(20) NOT NULL DEFAULT 'Aberta',
		criado_em TIMESTAMP NOT NULL DEFAULT NOW()
		)
		`)
	if err != nil {
		return err
	}

	// data_emissao so e preenchida quando a nota e impressa com sucesso
	// (vira 'Fechada'); fica nula enquanto a nota estiver 'Aberta'.
	if _, err := db.Exec(`ALTER TABLE notas ADD COLUMN IF NOT EXISTS data_emissao TIMESTAMP`); err != nil {
		return err
	}

	// sequencia usada para gerar o numero da nota fiscal automaticamente (NF-001, NF-002...)
	_, err = db.Exec(`CREATE SEQUENCE IF NOT EXISTS notas_numero_seq START 1`)
	return err
}
