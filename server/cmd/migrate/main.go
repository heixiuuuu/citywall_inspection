package main

import (
	"database/sql"
	"flag"
	"log"
	"strings"

	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/database"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	configPath := flag.String("config", "conf/server.json", "path to server config")
	directory := flag.String("dir", "migrations", "directory containing SQL migrations")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	dsn := cfg.Database.DSN()
	if !strings.Contains(dsn, "multiStatements") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		dsn += separator + "multiStatements=true"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db, *directory); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations applied")
}
