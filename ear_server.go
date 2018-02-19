package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo"
	"github.com/thebigear/controllers"
	"github.com/thebigear/database"
	"github.com/tuvistavie/structomap"
)

func init() {
	database.Connect()
	database.EnsureIndexes()
	// Use snake case in all serializers
	structomap.SetDefaultCase(structomap.SnakeCase)

	// Init the ear
	//go twitterear.TwitterEarConnect()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.POST("/expressions", controllers.CreateExpression)
	e.GET("/expressions", controllers.ListExpressions)
	e.PUT("/expressions/:id", controllers.UpdateExpression)
	e.DELETE("/expressions/:id", controllers.DeleteExpression)
	e.Logger.Fatal(e.Start(":1323"))
}
