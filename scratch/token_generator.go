package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secret := []byte("supersecret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  "d1d93333-3333-3333-3333-333333333333",
		"username": "test_user",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(secret)
	if err != nil {
		fmt.Printf("Error signing token: %v\n", err)
		return
	}
	fmt.Println(tokenString)
}
