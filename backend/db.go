package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db          *gorm.DB
	mongoCol    *mongo.Collection
	redisClient *redis.Client
)

func InitPostgres() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("[FATAL] Failed to connect to PostgreSQL: ", err)
	}

	log.Println(" Connected to PostgreSQL")

	err = db.AutoMigrate(&WorkItem{}, &RCA{})
	if err != nil {
		log.Fatal("[FATAL] Failed to auto-migrate PostgreSQL schemas: ", err)
	}
}

func InitMongoDB() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal("[FATAL] Failed to connect to MongoDB: ", err)
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("[FATAL] MongoDB ping failed: ", err)
	}

	log.Println(" Connected to MongoDB")
	mongoCol = client.Database("ims_db").Collection("signals")

	//  FIX 23: MongoDB Optimizations (Indexes & TTL)
	// 1. Compound index for fast timeline queries
	// 2. TTL (Time-To-Live) set to 30 days (2592000 seconds) to prevent infinite disk growth
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "timestamp", Value: 1},
			{Key: "component_id", Value: 1},
		},
		Options: options.Index().SetExpireAfterSeconds(2592000),
	}
	_, err = mongoCol.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Printf("[WARN] Could not create Mongo TTL/Indexes: %v\n", err)
	}
}

func InitRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("[FATAL] Failed to connect to Redis: ", err)
	}

	log.Println(" Connected to Redis")
}
