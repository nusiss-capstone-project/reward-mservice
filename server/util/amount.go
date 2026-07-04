package util

import (
	"errors"
	"math/big"
	"strings"
)

var ErrInvalidAmount = errors.New("invalid amount")

func ParseAmount(raw string) (*big.Rat, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, ErrInvalidAmount
	}
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok {
		return nil, ErrInvalidAmount
	}
	if amount.Sign() < 0 {
		return nil, ErrInvalidAmount
	}
	return amount, nil
}

func FormatAmount(amount *big.Rat) string {
	return amount.FloatString(8)
}

func AddAmount(left, right string) (string, error) {
	a, err := ParseAmount(left)
	if err != nil {
		return "", err
	}
	b, err := ParseAmount(right)
	if err != nil {
		return "", err
	}
	return FormatAmount(new(big.Rat).Add(a, b)), nil
}

func CmpAmount(left, right string) (int, error) {
	a, err := ParseAmount(left)
	if err != nil {
		return 0, err
	}
	b, err := ParseAmount(right)
	if err != nil {
		return 0, err
	}
	return a.Cmp(b), nil
}
