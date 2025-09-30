package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	//api "github.com/quackduck/devzat/devzatapi"
)

type FileData struct {
	name    string
	created time.Time
	size    int
}

type PluginConfiguration struct {
	MaxStorageSize      int
	MaxFileSize         int
	FileKeepingDuration time.Duration
	StoragePath         string
}

var (
	idMap  = make(map[string]FileData)
	lock   sync.Mutex
	config PluginConfiguration
)

func loadConfiguration(filePath string) {
	// Set default values
	config = PluginConfiguration{
		MaxStorageSize:      1 << 30,        // 1 GiB
		MaxFileSize:         256 << 20,      // 256 MiB
		FileKeepingDuration: 10 * time.Minute,
		StoragePath:         "./storage",
	}

	// Read the JSON file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to read config file: %v. Using default configuration.\n", err)
		return
	}

	// Parse JSON into a temporary struct to handle missing fields
	var jsonConfig PluginConfiguration
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		fmt.Printf("Failed to parse JSON config: %v. Using default configuration.\n", err)
		return
	}

	// Override defaults with values from JSON (only if they are not zero values)
	if jsonConfig.MaxStorageSize != 0 {
		config.MaxStorageSize = jsonConfig.MaxStorageSize
	}
	if jsonConfig.MaxFileSize != 0 {
		config.MaxFileSize = jsonConfig.MaxFileSize
	}
	if jsonConfig.FileKeepingDuration != 0 {
		config.FileKeepingDuration = jsonConfig.FileKeepingDuration
	}
	if jsonConfig.StoragePath != "" {
		config.StoragePath = jsonConfig.StoragePath
	}
}

func cleanupOldFiles() {
	lock.Lock()
	defer lock.Unlock()

	cutoffTime := time.Now().Add(-config.FileKeepingDuration)
	
	for fileId, fileData := range idMap {
		if fileData.created.Before(cutoffTime) {
			// Remove the file and directory from storage
			filePath := config.StoragePath + "/" + fileId
			err := os.RemoveAll(filePath)
			if err != nil {
				fmt.Printf("Failed to remove file directory %s: %v\n", filePath, err)
			}
			
			// Remove from idMap
			delete(idMap, fileId)
		}
	}
}

func webserver() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")

	router.GET("/request/:fileId", func(c *gin.Context) {
		// Clean up old files before serving the request
		cleanupOldFiles()
		
		fileId := c.Param("fileId")
		fmt.Println(fileId)

		c.HTML(http.StatusOK, "file-input.html", gin.H{
			"FileID": fileId,
		})
	})

	router.GET("/view/:fileId/", func(c *gin.Context) {
		fileId := c.Param("fileId")
		lock.Lock()
		filename := idMap[fileId].name
		lock.Unlock()
		c.File(config.StoragePath + "/" + fileId + "/" + filename)
	})

	router.POST("/upload/:fileId", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(config.MaxFileSize))
		fileId := c.Param("fileId")

		err := c.Request.ParseMultipartForm(int64(config.MaxFileSize))
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "File too large!")
			return
		}

		file, err := c.FormFile("filename")
		if err != nil {
			c.String(http.StatusBadRequest, "File upload error: %s", err.Error())
			return
		}

		// Check if adding this file would exceed the storage size limit
		lock.Lock()
		currentStorageSize := 0
		for _, fileData := range idMap {
			currentStorageSize += fileData.size
		}
		
		if currentStorageSize+int(file.Size) > config.MaxStorageSize {
			lock.Unlock()
			c.String(http.StatusInsufficientStorage, "Storage limit exceeded!")
			return
		}
		lock.Unlock()

		err = os.Mkdir(config.StoragePath+"/"+fileId, os.ModePerm)
		if err != nil {
			c.String(http.StatusBadRequest, "Dir creation error: %s", err.Error())
			return
		}

		err = c.SaveUploadedFile(file, config.StoragePath+"/"+fileId+"/"+file.Filename)
		if err != nil {
			c.String(http.StatusInternalServerError, "Unable to save file: %s", err.Error())
			return
		}

		lock.Lock()
		idMap[fileId] = FileData{name: file.Filename, created: time.Now(), size: int(file.Size)}
		lock.Unlock()

		c.String(http.StatusOK, "File %s uploaded successfully.", file.Filename)
	})

	router.Run("localhost:8080")
}

func main() {
	// Load configuration as first action
	if len(os.Args) < 2 {
		fmt.Println("Error: No configuration file path provided as argument. Using default configuration.")
		loadConfiguration("")
	} else {
		loadConfiguration(os.Args[1])
	}

	fmt.Println("Coucou")
	webserver()
}
