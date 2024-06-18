package handlers

import (
	"context"
	"net/http"
	"time"
	"tronicscorp/config"
	"tronicscorp/dbiface"

	"github.com/golang-jwt/jwt"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Email    string `json:"username" bson:"username" validate:"required,email"`
	Password string `json:"passwprd" bson:"password" validate:"required, min=8, max=300"`
	IsAdmin  bool   `json:"isadmin,omitempty" bson:"isadmin"`
}

type UsersHandler struct {
	Col dbiface.CollectionAPI
}

type errorMessage struct {
	Message string `json:"message"`
}

var (
	prop config.Properties
)

func isCredValid(givenPwd, storedPwd string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(storedPwd), []byte(givenPwd)); err != nil {
		return false
	}
	return true
}

func (u User) createToken() (string, error) {
	if err := cleanenv.ReadEnv(&prop); err != nil {
		log.Fatalf("Configuration cannot be read : %v", err)
	}
	claims := jwt.MapClaims{}
	claims["authorized"] = u.IsAdmin
	claims["user_id"] = u.Email
	claims["exp"] = time.Now().Add(time.Minute * 15).Unix()
	at := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token, err := at.SignedString([]byte(prop.JwtTokenSecret))
	if err != nil {
		log.Errorf("Unable to generate the token :%v", err)
		return "", err
	}
	return token, nil
}

func insertUser(ctx context.Context, user User, collection dbiface.CollectionAPI) (User, *echo.HTTPError) {
	var newUser User
	res := collection.FindOne(ctx, bson.M{"username": user.Email})
	err := res.Decode(&newUser)
	if err != nil && err != mongo.ErrNoDocuments {
		log.Errorf("Unable to decode retrieved user: %v", err)
		return newUser, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{"Unable to decode retrieved user"})
	}
	if newUser.Email != "" {
		log.Errorf("User by %s already exists", user.Email)
		return newUser, echo.NewHTTPError(http.StatusBadRequest, errorMessage{"User already exists"})
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 8)
	if err != nil {
		log.Errorf("Unable to hash the password: %v", err)
		return newUser, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{"Unable to process the password"})
	}
	user.Password = string(hashedPassword)
	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		log.Errorf("Unable to insert the user :%+v", err)
		return newUser, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{"Unable to create the user"})
	}
	return User{Email: user.Email}, nil
}
func authenticateUser(ctx context.Context, reqUser User, collection dbiface.CollectionAPI) (User, *echo.HTTPError) {
	var storedUser User
	res := collection.FindOne(ctx, bson.M{"username": reqUser.Email})
	err := res.Decode(&storedUser)
	if err != nil && err != mongo.ErrNoDocuments {
		log.Errorf("Unable to decode retrieved user: %v", err)
		return storedUser, echo.NewHTTPError(http.StatusUnprocessableEntity, "Unable to decode retrieved user")
	}
	if err == mongo.ErrNoDocuments {
		log.Errorf("User %s does not exist.", reqUser.Email)
		return storedUser, echo.NewHTTPError(http.StatusNotFound, "User does not exist")
	}
	if !isCredValid(reqUser.Password, storedUser.Password) {
		return storedUser, echo.NewHTTPError(http.StatusUnauthorized, "Credentials invalid")
	}
	return User{Email: storedUser.Email}, nil
}

func (h *UsersHandler) AuthnUser(c echo.Context) error {
	var user User
	c.Echo().Validator = &userValidator{validator: v}
	if err := c.Bind(&user); err != nil {
		log.Errorf("Unable to bind to user struct.")
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Unable to parse the request payload.")
	}
	if err := c.Validate(user); err != nil {
		log.Errorf("Unable to validate the requested body.")
		return echo.NewHTTPError(http.StatusBadRequest, "Unable to validate request payload.")
	}
	user, err := authenticateUser(context.Background(), user, h.Col)
	if err != nil {
		log.Errorf("Unable to authenticate to database.")
		return err
	}
	token, er := user.createToken()
	if er != nil {
		log.Errorf("Unable to generate the token.")
		return echo.NewHTTPError(http.StatusInternalServerError, "Unable to generate the token")
	}
	c.Response().Header().Set("x-auth-token", "Bearer"+token)
	return c.JSON(http.StatusOK, User{Email: user.Email})
}

func (h *UsersHandler) CreateUser(c echo.Context) error {
	var user User
	c.Echo().Validator = &userValidator{validator: v}
	if err := c.Bind(&user); err != nil {
		log.Errorf("Unable to bind to user struct.")
		return c.JSON(http.StatusUnprocessableEntity, errorMessage{"Unable to parse the request payload."})
	}
	if err := c.Validate(user); err != nil {
		log.Errorf("Unable to validate the requested body.")
		c.JSON(http.StatusBadRequest, errorMessage{"Unable to validate request body"})
	}
	resUser, httpError := insertUser(context.Background(), user, h.Col)
	if httpError != nil {
		return c.JSON(httpError.Code, httpError.Message)
	}
	token, err := user.createToken()
	if err != nil {
		log.Errorf("Unable to generate the token")
		return c.JSON(http.StatusInternalServerError, errorMessage{"Unable to generate the token"})
	}
	c.Response().Header().Set("x-auth-token", "Bearer"+token)
	return c.JSON(http.StatusCreated, resUser)
}
