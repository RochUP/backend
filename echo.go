package main

import (
	"net/http"

	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
)

type Result struct {
	Result bool `json:"result"`
}

func initRouting(e *echo.Echo, hub *Hub, db *gorm.DB) {

	e.GET("/", func(c echo.Context) error {
		serveHome(c.Response(), c.Request())
		return nil
	})

	e.POST("/user/signup", func(c echo.Context) error {
		result := &Result{
			Result: signupUser(db, c.FormValue("userId"), c.FormValue("userName"), c.FormValue("userPassword")),
		}

		return c.JSON(http.StatusOK, result)
	})

	e.POST("/user/login", func(c echo.Context) error {
		result := &Result{
			Result: loginUser(db, c.FormValue("userId"), c.FormValue("userPassword")),
		}

		return c.JSON(http.StatusOK, result)
	})

	e.GET("/ws", func(c echo.Context) error {
		serveWs(hub, c.Response(), c.Request())
		return nil
	})
}
