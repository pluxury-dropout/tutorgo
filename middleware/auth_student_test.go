package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func signToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func runWith(mw gin.HandlerFunc, token string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", mw, func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthStudent_AcceptsStudentToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "stu-1", "role": "student"})
	if w := runWith(AuthStudent(testSecret), tok); w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestAuthStudent_RejectsTutorToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "tut-1"}) // без роли = репетитор
	if w := runWith(AuthStudent(testSecret), tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_RejectsStudentToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "stu-1", "role": "student"})
	if w := runWith(Auth(testSecret), tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_AcceptsTutorToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "tut-1"})
	if w := runWith(Auth(testSecret), tok); w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
