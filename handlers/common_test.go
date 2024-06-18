package handlers

import (
	"context"
	"log"
	"os"
	"testing"
	"tronicscorp/config"

	"github.com/ilyakaznacheev/cleanenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	c        *mongo.Client
	db       *mongo.Database
	col      *mongo.Collection
	usersCol *mongo.Collection
	cfg      config.Properties
	h        ProductHandler
	uh       UsersHandler
)

func init() {
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("Unable to connect to database : %v", err)
	}
	db = c.Database(cfg.DBName)
	col = db.Collection(cfg.ProductCollection)
	usersCol = db.Collection(cfg.UsersCollection)
	isUserIndexUnique := true
	indexModel := mongo.IndexModel{
		Keys: bson.D{{"username", 1}},
		Options: &options.IndexOptions{
			Unique: &isUserIndexUnique,
		},
	}
	_, err := usersCol.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Fatalf("Unable to create an index : %+v", err)
	}
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	//set up
	testCode := m.Run()
	//destory
	usersCol.Drop(ctx)
	col.Drop(ctx)
	db.Drop(ctx)
	os.Exit(testCode)
}
