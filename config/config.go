package config

import (
	"os"
)

var (
	Host      = os.Getenv("host")
	Password  = os.Getenv("password")
	Port      = 5432
	User      = "postgres"
	Database  = "postgres"
	SecretKey = []byte(os.Getenv("SECRET_KEY"))
)
