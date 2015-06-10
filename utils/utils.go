package utils

import "crypto/rand"

func Uid(l int) string {
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	bytes := make([]byte, l)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	str := make([]rune, len(bytes))
	for i := range bytes {
		str[i] = alphabet[int(bytes[i]) % len(alphabet)]
	}
	return string(str)
}
