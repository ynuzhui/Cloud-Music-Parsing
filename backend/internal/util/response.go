package util

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func Err(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"code": status,
		"msg":  msg,
	})
}
