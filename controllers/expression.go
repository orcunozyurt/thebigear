package controllers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/thebigear/database"
	"github.com/thebigear/models"
)

// CreateExpression handles expression creation
func CreateExpression(c echo.Context) error {
	expression := &models.Expression{}
	if err := c.Bind(expression); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	expressionCreated, err := expression.Create()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	serializer := models.NewExpressionSerializer()
	c.Response().Header().Set("Location", fmt.Sprintf("%v/%v", c.Request().URL.Path, expressionCreated.URLToken))
	return c.JSON(http.StatusCreated, serializer.Transform(*expressionCreated))
}

// UpdateExpression updates menu title with :token
func UpdateExpression(c echo.Context) error {
	query := database.Query{}
	query["token"] = c.Param("expression_id")

	expression, err := models.GetExpression(query)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	if err := c.Bind(&expression); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	_, err = expression.Update()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	json := models.NewExpressionSerializer().Transform(*expression)
	return c.JSON(http.StatusOK, json)
}

// GetExpression gets menu title with :token
func GetExpression(c echo.Context) error {
	query := database.Query{}
	query["deleted_at"] = nil
	query["token"] = c.Param("expression_id")

	expression, err := models.GetExpression(query)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, models.NewExpressionSerializer().Transform(*expression))
}

// ListExpressions all menu titles
func ListExpressions(c echo.Context) error {
	query := database.Query{}
	query["deleted_at"] = nil

	expressions := &models.Expressions{}
	paginationParams := database.PaginationParamsForContext(c.QueryParam("page"),
		c.QueryParam("limit"), c.QueryParam("sort_by"))

	if c.QueryParam("owner") != "" {
		regexQuery := database.Query{}
		regexQuery["$regex"] = c.QueryParam("owner")
		regexQuery["$options"] = "i"
		query["owner"] = regexQuery
	}

	expressions, err := models.ListExpressions(query, paginationParams)
	if err != nil {
		return err
	}

	json, _ := models.NewExpressionSerializer().TransformArray(*expressions)
	return c.JSON(http.StatusOK, json)
}

// DeleteExpression deletes menu title with :token
func DeleteExpression(c echo.Context) error {
	query := database.Query{}
	query["token"] = c.Param("expression_id")

	expression, err := models.GetExpression(query)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	if err := expression.Delete(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	c.Response().WriteHeader(http.StatusNoContent)
	return nil
}
