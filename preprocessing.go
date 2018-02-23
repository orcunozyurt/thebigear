package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/thebigear/database"
	"github.com/thebigear/models"
	"github.com/tuvistavie/structomap"
)

func init() {
	database.Connect()
	database.EnsureIndexes()
	// Use snake case in all serializers
	structomap.SetDefaultCase(structomap.SnakeCase)
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
}

func main() {

	query := database.Query{}
	query["deleted_at"] = nil

	expressions := &models.Expressions{}

	paginationParams := database.PaginationParamsForContext("", "", "")

	expressions, _ = models.ListExpressions(query, paginationParams)

	fmt.Println("TOTAL OF:", len(*expressions))

}
