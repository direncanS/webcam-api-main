package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/lambda-lama/webcam-api/config"
)

func GetConnection() (*pgx.Conn, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", config.User, config.Password, config.Host, config.Port, config.Database)

	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		return nil, err
	}
	return conn, nil
}
