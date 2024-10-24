package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"
)

// Structs for configuration file
type Config struct {
	App     AppConfig     `yaml:"app"`
	MongoDB MongoDBConfig `yaml:"mongodb"`
	Logging LoggingConfig `yaml:"logging"`
}

type AppConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type MongoDBConfig struct {
	URL        string `yaml:"url"`
	Database   string `yaml:"database"`
	Collection string `yaml:"collection"`
}

type LoggingConfig struct {
	FilePath  string `yaml:"file_path"`
	Rotation  string `yaml:"rotation"`
	Retention int    `yaml:"retention"`
}

var config Config // Configuration variable
var mongoClient *mongo.Client

// Function to connect to MongoDB
func connectToMongoDB() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(config.MongoDB.URL)
	return mongo.Connect(context.TODO(), clientOptions)
}

func loadConfig() error {
	configFile := "./config.yaml"
	log.Printf("Attempting to load config from: %s", configFile)

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf("Config file not found at %s, using default values", configFile)
		config = Config{
			App:     AppConfig{Host: "localhost", Port: "8080"},
			MongoDB: MongoDBConfig{URL: "mongodb://localhost:27017", Database: "http_hopper", Collection: "destinations"},
			Logging: LoggingConfig{FilePath: "app.log", Retention: 7},
		}
		log.Printf("Default configuration: %+v", config)
		return nil
	}

	// Read and parse the configuration file
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Error parsing config file: %v", err)
		return fmt.Errorf("error parsing config file: %v", err)
	}

	// Validate configuration
	if config.App.Host == "" || config.App.Port == "" {
		log.Printf("Invalid App configuration: Host and Port must be specified")
		return fmt.Errorf("invalid App configuration: Host and Port must be specified")
	}
	if config.MongoDB.URL == "" || config.MongoDB.Database == "" || config.MongoDB.Collection == "" {
		log.Printf("Invalid MongoDB configuration: URL, Database, and Collection must be specified")
		return fmt.Errorf("invalid MongoDB configuration: URL, Database, and Collection must be specified")
	}

	log.Printf("Configuration loaded successfully: %+v", config)
	return nil
}

func main() {
	fmt.Println("Starting main function...")

	// Load config from YAML file
	log.Println("Loading configuration...")
	if err := loadConfig(); err != nil {
		log.Printf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Set up initial error logging to a file
	errorLogFile, err := os.OpenFile("error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open error log file: %v\n", err)
		os.Exit(1)
	}
	defer errorLogFile.Close()

	// Set up multi-writer for logging
	multiWriter := io.MultiWriter(os.Stdout, errorLogFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("Error logging set up successfully")

	// MongoDB connection
	log.Println("Connecting to MongoDB...")
	mongoClient, err = connectToMongoDB() // Connect to MongoDB using the config data
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		os.Exit(1)
	}

	// Verify the connection by pinging the database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = mongoClient.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB after connection: %v", err)
	}
	log.Println("Successfully pinged MongoDB after connection")

	// Initialize router
	log.Println("Initializing router...")
	router := mux.NewRouter()
	router = initializeRoutes(router)

	// Create a new server
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", config.App.Host, config.App.Port),
		Handler: router,
	}

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting http hopper service on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop

	// Shutdown the server
	log.Println("Shutting down server...")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server gracefully stopped")
}

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func getCurrentDir() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("Error getting current directory: %v", err)
		return ""
	}
	return dir
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
