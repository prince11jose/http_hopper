package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/gorilla/mux"
	"github.com/natefinch/lumberjack"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

var config Config
var mongoClient *mongo.Client

func loadConfig() error {
	configFile := "./config.yaml"
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf("Config file not found at %s, using default values", configFile)
		config = Config{
			App:     AppConfig{Host: "localhost", Port: "8080"},
			MongoDB: MongoDBConfig{URL: "mongodb://localhost:27017"},
			Logging: LoggingConfig{FilePath: "app.log", Retention: 7},
		}
		return nil
	}
	// Check file permissions
	info, err := os.Stat(configFile)
	if err != nil {
		return fmt.Errorf("error checking config file permissions: %v", err)
	}
	if info.Mode().Perm()&0444 == 0 {
		return fmt.Errorf("config file is not readable")
	}
	// Read and parse the configuration file
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}
	log.Printf("Configuration loaded successfully: %+v", config)
	return nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
	}()

	var err error // Declare err here

	// Check if all necessary files are present
	requiredFiles := []string{"main.go", "forwarder.go", "handlers.go", "logger.go", "mongodb.go", "router.go", "config.yaml"}
	for _, file := range requiredFiles {
		if _, err = os.Stat(file); os.IsNotExist(err) {
			log.Fatalf("Required file %s is missing", file)
		}
	}

	// Initial logging setup (before loading config)
	log.SetOutput(&lumberjack.Logger{
		Filename: "app.log",
		MaxSize:  10,
		MaxAge:   7,
		Compress: true,
	})

	log.Println("Starting application...")

	// Load config from YAML file
	log.Println("Loading configuration...")
	if err = loadConfig(); err != nil { // Use err here
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Update logging with config values
	log.SetOutput(&lumberjack.Logger{
		Filename: config.Logging.FilePath,
		MaxSize:  10,
		MaxAge:   config.Logging.Retention,
		Compress: true,
	})

	// MongoDB connection
	log.Println("Connecting to MongoDB...")
	log.Printf("MongoDB URL: %s", config.MongoDB.URL)
	clientOptions := options.Client().ApplyURI(config.MongoDB.URL)

	// Retry mechanism for MongoDB connection
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		mongoClient, err = mongo.Connect(ctx, clientOptions) // Use mongoClient here
		cancel()

		if err == nil {
			// Ping the database
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			err = mongoClient.Ping(ctx, nil)
			cancel()

			if err == nil {
				break
			}
		}

		log.Printf("Failed to connect to MongoDB (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Printf("Failed to connect to MongoDB after %d attempts: %v", maxRetries, err)
		log.Println("Please ensure MongoDB is running and accessible")
		os.Exit(1)
	}

	log.Println("Successfully connected to MongoDB")

	// Check if the required collection exists
	collections, err := mongoClient.Database(config.MongoDB.Database).ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		log.Printf("Failed to list collections: %v", err)
		os.Exit(1)
	}
	log.Printf("Available collections: %v", collections)

	// If the destinations collection doesn't exist, create it
	if !contains(collections, config.MongoDB.Collection) {
		err = mongoClient.Database(config.MongoDB.Database).CreateCollection(context.Background(), config.MongoDB.Collection)
		if err != nil {
			log.Printf("Failed to create %s collection: %v", config.MongoDB.Collection, err)
			os.Exit(1)
		}
		log.Printf("Created %s collection", config.MongoDB.Collection)
	}

	// Ensure MongoDB client is properly closed on exit
	defer func() {
		if err = mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Set up the router and routes
	log.Println("Setting up routes...")
	r := mux.NewRouter()
	initializeRoutes(r)

	// Create a new server
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", config.App.Host, config.App.Port),
		Handler: r,
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
