package model

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
)

var (
	db   *sql.DB
	once sync.Once
)

func MakeMySQLEndpoint(conf *config.MySQLConfig) string {
	params := make(map[string]string)
	params["parseTime"] = "true"
	ep := fmt.Sprintf("%s:%s@tcp(%s)/%s?", conf.User, conf.Password, conf.Host, conf.Database)
	for k, v := range params {
		dsn := fmt.Sprintf("%s=%s&", k, v)
		ep += dsn
	}
	return ep
}

func CreateMySQLClient(conf *config.MySQLConfig) error {
	var err error
	once.Do(func() {
		endpoint := MakeMySQLEndpoint(conf)
		db, err = sql.Open("mysql", endpoint)
		db.SetConnMaxLifetime(30 * time.Second)
	})
	return err
}

func getDB() *sql.DB {
	return db
}
