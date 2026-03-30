package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Our Todo model -- like a Mongoose schema
type Todo struct {
	ID   primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Task string             `json:"task" bson:"task"`
	Done bool               `json:"done" bson:"done"`
}

// Package-level variable to hold the collection -- like:
// const Todo = mongoose.model("Todo", todoSchema)
var collection *mongo.Collection

func main() {
	// -----------------------------------------------
	// Step 1: Connect to MongoDB
	// -----------------------------------------------
	// Load .env file if present (like require('dotenv').config() in Node)
	godotenv.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI environment variable is not set")
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Verify the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Failed to ping MongoDB:", err)
	}
	fmt.Println("Connected to MongoDB!")

	// Get a handle to the "todos" collection in the "lesson3" database
	collection = client.Database("lesson3").Collection("todos")

	// -----------------------------------------------
	// Routes
	// -----------------------------------------------
	http.HandleFunc("/todos", handleTodos)
	http.HandleFunc("/todos/", handleTodoByID)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// GET /todos  -> list all todos
// POST /todos -> create a todo
func handleTodos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		// Find all documents -- like: await Todo.find({})
		cursor, err := collection.Find(context.TODO(), bson.M{})
		if err != nil {
			http.Error(w, "Failed to fetch todos", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		// Decode all results into a slice (Go's array)
		var todos []Todo
		err = cursor.All(context.TODO(), &todos)
		if err != nil {
			http.Error(w, "Failed to decode todos", http.StatusInternalServerError)
			return
		}

		// Return empty array instead of null when no todos
		if todos == nil {
			todos = []Todo{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todos)

	case http.MethodPost:
		var todo Todo
		err := json.NewDecoder(r.Body).Decode(&todo)
		if err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Insert one document -- like: await Todo.create({ task, done })
		result, err := collection.InsertOne(context.TODO(), todo)
		if err != nil {
			http.Error(w, "Failed to insert todo", http.StatusInternalServerError)
			return
		}

		// Set the generated ID back on the struct
		todo.ID = result.InsertedID.(primitive.ObjectID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(todo)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// DELETE /todos/{id} -> delete a todo
func handleTodoByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL: "/todos/abc123" -> "abc123"
	// In Node/Express this would be req.params.id
	idStr := r.URL.Path[len("/todos/"):]

	// Convert string ID to MongoDB ObjectID
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Delete one document -- like: await Todo.findByIdAndDelete(id)
	result, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
	if err != nil {
		http.Error(w, "Failed to delete todo", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Todo deleted"})
}
