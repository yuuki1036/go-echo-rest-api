package main

import (
	"fmt"

	"go-echo-rest-api/db"
	"go-echo-rest-api/model"
)

func main() {
	dbConn := db.NewDB()
	defer fmt.Println("Successfully migrated")
	defer db.CloseDB(dbConn)
	dbConn.AutoMigrate(&model.User{}, &model.Task{})
}
