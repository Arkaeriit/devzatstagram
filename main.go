package main

import (
	"os"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	//api "github.com/quackduck/devzat/devzatapi"
)

func webserver() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/request/:fileId", func(c *gin.Context) {
		fileId := c.Param("fileId")
		fmt.Println(fileId)

		c.HTML(http.StatusOK, "file-input.html", gin.H{
			"FileID": fileId,
		})
	})
	
	router.GET("/view/:fileId/", func(c *gin.Context) {
		//fileId := c.Param("fileId")
		c.File("./uploads/test.jpg")
	})

	router.POST("/upload/:fileId", func(c *gin.Context) {
		fileId := c.Param("fileId")

        file, err := c.FormFile("filename")
        if err != nil {
            c.String(http.StatusBadRequest, "File upload error: %s", err.Error())
            return
        }

		err = os.Mkdir("./uploads/" + fileId, os.ModePerm)
        if err != nil {
            c.String(http.StatusBadRequest, "Dir creation error: %s", err.Error())
            return
        }

        err = c.SaveUploadedFile(file, "./uploads/" + fileId + "/" + file.Filename)
        if err != nil {
            c.String(http.StatusInternalServerError, "Unable to save file: %s", err.Error())
            return
        }

        c.String(http.StatusOK, "File %s uploaded successfully.", file.Filename)
    })

	router.Run("localhost:8080")
}

func main() {
	fmt.Println("Coucou")
	webserver()
}

