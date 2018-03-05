package main

import (
	"crypto/rand"
)

const (
	upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lower   = "abcdefghijklmnopqrstuvwxyz"
	number  = "0123456789"
	special = "@%-_+,./:"
	chars   = upper + lower + number + special
)

type PasswordGenerator func(int) string

func GenerateSecurePassword(n int) string {
	for {
		p, err := generatePassword(n)
		if err != nil {
			continue
		}
		if p.Password[0] != '-' &&
			(p.Upper > 0 || p.Lower > 0 || p.Number > 0 || p.Special > 0) {
			return p.Password
		}
	}
}

type password struct {
	Password string
	Upper    int
	Lower    int
	Number   int
	Special  int
}

func (p *password) AddChar(idx int) {
	p.Password = p.Password + string(chars[idx])
	switch {
	case idx < len(upper):
		p.Upper = p.Upper + 1
	case idx < len(upper)+len(lower):
		p.Lower = p.Lower + 1
	case idx < len(upper)+len(lower)+len(special):
		p.Number = p.Number + 1
	default:
		p.Special = p.Special + 1
	}
}

func generatePassword(n int) (password, error) {
	b, err := randomBytes(n)
	if err != nil {
		return password{}, err
	}
	p := password{}
	for _, char := range b {
		idx := int(char) % len(chars)
		p.AddChar(idx)
	}
	return p, nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
