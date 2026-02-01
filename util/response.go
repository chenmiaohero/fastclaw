package util

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func Error(c echo.Context, code int, message string) error {
	return c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

func BadRequest(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Message: message,
	})
}

func NotFound(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, Response{
		Code:    404,
		Message: message,
	})
}

func InternalError(c echo.Context, message string) error {
	return c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: message,
	})
}

func Unauthorized(c echo.Context, message string) error {
	return c.JSON(http.StatusUnauthorized, Response{
		Code:    401,
		Message: message,
	})
}
