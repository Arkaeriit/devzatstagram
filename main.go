package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	//api "github.com/quackduck/devzat/devzatapi"
)

func webserver() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.GET("/request/:fileId", func(c *gin.Context) {
		c.HTML(http.StatusOK, "file-input.html", gin.H{
			"fileId": c.Param("fileId"),
		})
	})
	router.Run("localhost:8080")
}

func main() {
	fmt.Println("Coucou")
	webserver()
}

