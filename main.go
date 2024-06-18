package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"tronicscorp/config"
	"tronicscorp/handlers"

	"github.com/golang-jwt/jwt"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/labstack/gommon/random"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	CorrelationID = "X-Correlation-ID"
)

var (
	c        *mongo.Client
	db       *mongo.Database
	prodCol  *mongo.Collection
	usersCol *mongo.Collection
	cfg      config.Properties
)

func init() {
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("Configurations cannot be read: %v", err)
	}

	connectURI := fmt.Sprintf("mongodb://%s:%s", cfg.DBHost, cfg.DBPort)
	c, err := mongo.Connect(context.Background(), options.Client().ApplyURI(connectURI))
	if err != nil {
		log.Fatalf("Unable to connect to database : %v", err)
	}

	db = c.Database(cfg.DBName)
	prodCol = db.Collection(cfg.ProductCollection)
	usersCol = db.Collection(cfg.UsersCollection)

	isUserIndexUnique := true
	indexModel := mongo.IndexModel{
		Keys: bson.D{{"username", 1}},
		Options: &options.IndexOptions{
			Unique: &isUserIndexUnique,
		},
	}
	_, err = usersCol.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Fatalf("Unable to create an index : %+v", err)
	}
}

func addCorrelationID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Request().Header.Get(CorrelationID)
		var newID string
		if id == "" {
			newID = random.String(12)
		} else {
			newID = id
		}
		c.Request().Header.Set(CorrelationID, newID)
		c.Response().Header().Set(CorrelationID, newID)
		return next(c)
	}
}

func adminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		hToken := c.Request().Header.Get("x-auth-token")
		token := strings.Split(hToken, "")[1]
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
			return []byte(cfg.JwtTokenSecret), nil
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to parse token")
		}
		if !claims["authorized"].(bool) {
			return echo.NewHTTPError(http.StatusForbidden, "Not authorized")
		}
		return next(c)
	}
}

func main() {
	e := echo.New()
	e.Logger.SetLevel(log.DEBUG)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(addCorrelationID)
	jwtMiddleware := middleware.JWTWithConfig(middleware.JWTConfig{
		SigningKey:  []byte(cfg.JwtTokenSecret),
		TokenLookup: "Header:x-auth-token",
	})
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${host} ${method} ${uri} ${user_agent}` +
			`${status} ${error} ${latency_human}` + "\n",
	}))
	h := &handlers.ProductHandler{Col: prodCol}
	uh := &handlers.UsersHandler{Col: usersCol}
	e.GET("/product/:id", h.GetProduct)
	e.DELETE("/product/:id", h.DeleteProduct, jwtMiddleware, adminMiddleware)
	e.PUT("/products/:id", h.UpdateProduct, middleware.BodyLimit("1M"), jwtMiddleware)
	e.POST("/products", h.CreateProducts, middleware.BodyLimit("1M"), jwtMiddleware)
	e.GET("/products", h.GetProducts)

	e.POST("/users", uh.CreateUser)
	e.POST("/auth", uh.AuthnUser)
	e.Logger.Infof("Listening on %s:%s", cfg.Host, cfg.Port)
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)))
}
