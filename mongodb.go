package main

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func getAllDestinationsFromDB() ([]Destination, error) {
	collection := mongoClient.Database("http_hopper").Collection("destinations")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("MongoDB Find Error: %v", err)
	}
	var destinations []Destination
	if err = cursor.All(context.TODO(), &destinations); err != nil {
		return nil, fmt.Errorf("MongoDB Cursor Error: %v", err)
	}
	return destinations, nil
}

func addDestinationToDB(destination Destination) {
	collection := mongoClient.Database("http_hopper").Collection("destinations")
	_, err := collection.InsertOne(context.TODO(), destination)
	if err != nil {
		log.Fatal("MongoDB Insert Error:", err)
	}
}

func updateDestinationInDB(id string, updatedDestination Destination) {
	collection := mongoClient.Database("http_hopper").Collection("destinations")

	// Convert the ID string to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Invalid ID format: %v", err)
		return
	}

	// Prepare the update document
	update := bson.M{}
	if updatedDestination.URL != "" {
		update["url"] = updatedDestination.URL
	}
	update["isActive"] = updatedDestination.IsActive // Always update isActive
	if updatedDestination.Method != "" {
		update["method"] = updatedDestination.Method
	}

	// Perform the update operation
	result, err := collection.UpdateOne(context.TODO(), bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		log.Printf("MongoDB Update Error: %v", err)
		return
	}

	if result.MatchedCount == 0 {
		log.Printf("No document found with ID: %s", id)
	} else {
		log.Printf("Updated document with ID: %s", id)
	}
}

func deleteDestinationFromDB(id string) {
	collection := mongoClient.Database("http_hopper").Collection("destinations")

	// Convert the ID string to ObjectID if you're using ObjectID in MongoDB
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Fatalf("Invalid ID format: %v", err)
		return
	}

	// Delete the document with the matching ObjectID
	result, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objectID})
	if err != nil {
		log.Fatal("MongoDB Delete Error:", err)
		return
	}

	if result.DeletedCount == 0 {
		log.Printf("No document found with ID: %s", id)
	} else {
		log.Printf("Deleted document with ID: %s", id)
	}
}
