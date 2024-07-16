package utils

import (
	"errors"

	"github.com/matthewhartstonge/argon2"
)

var argon2Config = argon2.Config{
	HashLength:  32, // 32 * 8 = 256-bits
	SaltLength:  16, // 16 * 8 = 128-bits
	TimeCost:    3,
	MemoryCost:  64 * 1024, // 64MB
	Parallelism: 1,
	Mode:        argon2.ModeArgon2id,
	Version:     argon2.Version13,
}

func CheckAndHashPassword(rawPassword string) (hashed string, err error) {
	b, err := argon2Config.HashEncoded([]byte(rawPassword))
	return string(b), err
}
func VerifyPassword(hash, rawPassword string) (err error) {
	match, err := argon2.VerifyEncoded([]byte(rawPassword), []byte(hash))
	if !match {
		err = errors.New("password mismatch")
	}
	return
}
