package model

import (
	"log"

	"github.com/rakutentech/shibuya/shibuya/config"
)

func setupAndTeardown() error {
	conf := &config.MySQLConfig{
		Host:     "localhost",
		User:     "root",
		Password: "root",
		Database: "shibuya",
	}
	if err := CreateMySQLClient(conf); err != nil {
		log.Fatal(err)
	}
	db := getDB()
	q, err := db.Prepare("delete from plan")
	if err != nil {
		return err
	}
	defer q.Close()
	_, err = q.Exec()
	if err != nil {
		return err
	}

	q, err = db.Prepare("delete from running_plan")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	q, err = db.Prepare("delete from collection")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	q, err = db.Prepare("delete from collection_plan")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	q, err = db.Prepare("delete from project")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	q, err = db.Prepare("delete from collection_run")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	q, err = db.Prepare("delete from collection_run_history")
	if err != nil {
		return err
	}
	_, err = q.Exec()
	if err != nil {
		return err
	}
	return nil
}
