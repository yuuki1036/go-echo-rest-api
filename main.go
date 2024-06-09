package main

import (
	"go-echo-rest-api/controller"
	"go-echo-rest-api/db"
	"go-echo-rest-api/repository"
	"go-echo-rest-api/router"
	"go-echo-rest-api/usecase"
)

func main() {
	db := db.NewDB()
	userRepository := repository.NewUserRepository(db)
	userUsecase := usecase.NewUserUsecase(userRepository)
	userController := controller.NewUserController(userUsecase)
	e := router.NewRouter(userController)
	e.Logger.Fatal(e.Start(":8080"))
}
