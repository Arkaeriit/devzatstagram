package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/quackduck/devzat/devzatapi"
)

type FileData struct {
	name    string
	created time.Time
	size    int
	usable  bool
}

type PluginConfiguration struct {
	MaxStorageSize      int
	MaxFileSize         int
	FileKeepingDuration time.Duration
	StoragePath         string
	DevzatToken         string
	DevzatHost          string
	WebHost             string
}

var (
	idMap         = make(map[string]FileData)
	lock          sync.Mutex
	config        PluginConfiguration
	devzatSession *api.Session
	devzatLock    sync.Mutex
)

func loadConfiguration(filePath string) {
	// Set default values
	config = PluginConfiguration{
		MaxStorageSize:      1 << 30,   // 1 GiB
		MaxFileSize:         256 << 20, // 256 MiB
		FileKeepingDuration: 10 * time.Minute,
		StoragePath:         "./storage",
		DevzatHost:          "devzat.hackclub.com:5556",
		WebHost:             "http://localhost:8080",
		// DevzatToken has no default - program will terminate if missing
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
	if jsonConfig.DevzatHost != "" {
		config.DevzatHost = jsonConfig.DevzatHost
	}
	if jsonConfig.DevzatToken != "" {
		config.DevzatToken = jsonConfig.DevzatToken
	}
	if jsonConfig.WebHost != "" {
		config.WebHost = jsonConfig.WebHost
	}

	// Check for required fields
	if config.DevzatToken == "" {
		fmt.Printf("Error: DevzatToken is required in configuration file.\n")
		os.Exit(1)
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

func formatFileSize(bytes int) string {
	const (
		KiB = 1024
		MiB = KiB * 1024
		GiB = MiB * 1024
	)

	if bytes >= GiB {
		return fmt.Sprintf("%d GiB", bytes/GiB)
	} else if bytes >= MiB {
		return fmt.Sprintf("%d MiB", bytes/MiB)
	} else if bytes >= KiB {
		return fmt.Sprintf("%d KiB", bytes/KiB)
	}
	return fmt.Sprintf("%d B", bytes)
}

func generateFileID() string {
	lock.Lock()
	defer lock.Unlock()

	// Start with 4 hex digits (2 bytes)
	for length := 2; length <= 16; length++ {
		// Generate random bytes
		bytes := make([]byte, length)
		_, err := rand.Read(bytes)
		if err != nil {
			continue // Try again on error
		}

		// Convert to hex string
		id := hex.EncodeToString(bytes)

		// Check if this ID is already used
		if _, exists := idMap[id]; !exists {
			return id
		}
	}

	// Fallback - should never happen with 16 bytes (32 hex chars)
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func checkFileUsable(fileId string) bool {
	lock.Lock()
	defer lock.Unlock()

	fileData, exists := idMap[fileId]
	if !exists {
		return false
	}
	return fileData.usable
}

func webserver() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	router.NoRoute(func(c *gin.Context) {
		c.File("./static/404.html")
	})

	router.GET("/request/:fileId/:username/:room", func(c *gin.Context) {
		// Clean up old files before serving the request
		cleanupOldFiles()

		fileId := c.Param("fileId")
		fmt.Println(fileId)

		if !checkFileUsable(fileId) {
			c.Status(http.StatusNotFound)
			c.File("./static/404.html")
			return
		}

		c.HTML(http.StatusOK, "file-input.html", gin.H{
			"FileID":      fileId,
			"Username":    c.Param("username"),
			"Room":        c.Param("room"),
			"MaxFileSize": formatFileSize(config.MaxFileSize),
		})
	})

	router.GET("/view/:fileId/:filename", func(c *gin.Context) {
		fileId := c.Param("fileId")
		// filename parameter is ignored
		lock.Lock()
		filename := idMap[fileId].name
		lock.Unlock()
		c.File(config.StoragePath + "/" + fileId + "/" + filename)
	})

	router.POST("/upload/:fileId/:username/:room", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(config.MaxFileSize))
		fileId := c.Param("fileId")
		username := c.Param("username")
		room := c.Param("room")

		if !checkFileUsable(fileId) {
			c.Status(http.StatusNotFound)
			c.File("./static/404.html")
			return
		}

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
			c.Redirect(http.StatusSeeOther, "/static/storage-full.html")
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
		idMap[fileId] = FileData{name: file.Filename, created: time.Now(), size: int(file.Size), usable: false}
		lock.Unlock()

		// Send message to the room with the view URL as markdown image
		// Truncate filename to last 20 characters if longer
		displayFilename := file.Filename
		if len(displayFilename) > 20 {
			displayFilename = displayFilename[len(displayFilename)-20:]
		}

		viewURL := config.WebHost + "/view/" + fileId + "/" + displayFilename
		markdownImage := fmt.Sprintf("![%s ](%s)", viewURL, viewURL)
		devzatLock.Lock()
		dmErr := devzatSession.SendMessage(api.Message{
			Room: "#" + room, // Add back the '#' prefix for devzat room
			From: username,
			Data: markdownImage,
		})
		devzatLock.Unlock()

		if dmErr != nil {
			fmt.Printf("Error sending message to room: %v\n", dmErr)
		}

		c.Redirect(http.StatusSeeOther, "/static/upload-success.html")
	})

	router.Run("localhost:8080")
}

func main() {
	// Configuration file is required
	if len(os.Args) < 2 {
		fmt.Println("Error: Configuration file path is required as argument.")
		os.Exit(1)
	}

	// Load configuration
	loadConfiguration(os.Args[1])

	// Initialize storage directory
	os.RemoveAll(config.StoragePath)
	os.Mkdir(config.StoragePath, os.ModePerm)

	// Connect to Devzat
	var err error
	devzatSession, err = api.NewSession(config.DevzatHost, config.DevzatToken)
	if err != nil {
		fmt.Printf("Error: Failed to connect to Devzat: %v\n", err)
		os.Exit(1)
	}

	// Register devzatstagram command
	err = devzatSession.RegisterCmd("devzatstagram", "", "Get a link to upload a picture", func(cmdCall api.CmdCall, err error) {
		devzatLock.Lock()
		defer devzatLock.Unlock()

		if err != nil {
			fmt.Printf("Error in devzatstagram command: %v\n", err)
			return
		}

		// Generate a unique file ID
		fileId := generateFileID()

		// Add to idMap with usable set to true
		lock.Lock()
		idMap[fileId] = FileData{usable: true, created: time.Now()}
		lock.Unlock()

		// Create the upload link
		room := cmdCall.Room
		if len(room) > 0 && room[0] == '#' {
			room = room[1:] // Remove leading '#'
		}
		uploadLink := config.WebHost + "/request/" + fileId + "/" + cmdCall.From + "/" + room

		// Send DM to the command sender
		dmErr := devzatSession.SendMessage(api.Message{
			Room: cmdCall.Room,
			DMTo: cmdCall.From,
			Data: fmt.Sprintf("Use this link to upload a picture: %s", uploadLink),
		})

		if dmErr != nil {
			fmt.Printf("Error sending DM: %v\n", dmErr)
		}
	})

	if err != nil {
		fmt.Printf("Error registering devzatstagram command: %v\n", err)
		os.Exit(1)
	}

	webserver()
}
