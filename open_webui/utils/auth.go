package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	ID  string `json:"id"`
	JTI string `json:"jti"`
	jwt.RegisteredClaims
}

func GetPasswordHash(password string) (string, error) {
	if len([]byte(password)) > 72 {
		return "", errors.New("password too long")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(plainPassword string, hashedPassword string) bool {
	if hashedPassword == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword)) == nil
}

func ValidateEmailFormat(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func ParseDurationSpec(value string) (time.Duration, bool, error) {
	if value == "-1" || value == "0" {
		return 0, false, nil
	}

	pattern := regexp.MustCompile(`(-?\d+(?:\.\d+)?)(ms|s|m|h|d|w)`)
	matches := pattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return 0, false, errors.New("invalid duration string")
	}

	var total time.Duration
	for _, match := range matches {
		number, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return 0, false, err
		}

		switch match[2] {
		case "ms":
			total += time.Duration(number * float64(time.Millisecond))
		case "s":
			total += time.Duration(number * float64(time.Second))
		case "m":
			total += time.Duration(number * float64(time.Minute))
		case "h":
			total += time.Duration(number * float64(time.Hour))
		case "d":
			total += time.Duration(number * float64(24*time.Hour))
		case "w":
			total += time.Duration(number * float64(7*24*time.Hour))
		default:
			return 0, false, errors.New("invalid duration unit")
		}
	}
	return total, true, nil
}

func CreateToken(secret string, userID string, expiresIn string, now time.Time) (string, *time.Time, error) {
	if strings.TrimSpace(secret) == "" {
		return "", nil, errors.New("secret is required")
	}

	claims := Claims{
		ID:  userID,
		JTI: strconv.FormatInt(now.UnixNano(), 10),
	}

	duration, hasExpiry, err := ParseDurationSpec(expiresIn)
	if err != nil {
		return "", nil, err
	}
	if hasExpiry {
		expiresAt := now.Add(duration)
		claims.ExpiresAt = jwt.NewNumericDate(expiresAt)
	}
	claims.IssuedAt = jwt.NewNumericDate(now)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", nil, err
	}

	if claims.ExpiresAt != nil {
		expiresAt := claims.ExpiresAt.Time
		return signed, &expiresAt, nil
	}
	return signed, nil, nil
}

func DecodeToken(secret string, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func ExtractTokenFromAuthHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func ExtractTokenFromRequest(r *http.Request) string {
	if token := ExtractTokenFromAuthHeader(r.Header.Get("Authorization")); token != "" {
		return token
	}
	cookie, err := r.Cookie("token")
	if err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func CreateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(buf), nil
}
