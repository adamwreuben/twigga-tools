package utils

import (
	"crypto/rand"
)

func GenerateDocumentID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	length := 20
	bytes := make([]byte, length)

	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}

	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}

	return string(bytes)
}
