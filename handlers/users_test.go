package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestUsers(t *testing.T) {
	t.Run("test create user invalid data unhappy", func(t *testing.T) {
		body := `{
			"username":"krunal.Shimpi@gmail.com",
			"Password": "abc12"
		}`
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
		res := httptest.NewRecorder()
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e := echo.New()
		c := e.NewContext(req, res)
		uh.Col = usersCol
		err := uh.CreateUser(c)
		t.Logf("res: %#+v\n", string(res.Body.Bytes()))
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
	})
}
