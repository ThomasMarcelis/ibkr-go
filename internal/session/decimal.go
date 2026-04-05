package session

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrInvalidDecimal = errors.New("session: invalid decimal")

type Decimal struct {
	text string
}

func ParseDecimal(raw string) (Decimal, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Decimal{}, ErrInvalidDecimal
	}

	neg := false
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		neg = true
		s = s[1:]
	}
	if s == "" {
		return Decimal{}, ErrInvalidDecimal
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return Decimal{}, ErrInvalidDecimal
	}

	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if intPart == "" {
		intPart = "0"
	}
	for _, r := range intPart {
		if r < '0' || r > '9' {
			return Decimal{}, ErrInvalidDecimal
		}
	}
	for _, r := range fracPart {
		if r < '0' || r > '9' {
			return Decimal{}, ErrInvalidDecimal
		}
	}

	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	fracPart = strings.TrimRight(fracPart, "0")

	text := intPart
	if fracPart != "" {
		text += "." + fracPart
	}
	if text == "0" {
		neg = false
	}
	if neg {
		text = "-" + text
	}
	return Decimal{text: text}, nil
}

func MustParseDecimal(raw string) Decimal {
	d, err := ParseDecimal(raw)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Decimal) String() string {
	if d.text == "" {
		return "0"
	}
	return d.text
}

func (d Decimal) IsZero() bool {
	return d.String() == "0"
}

func (d Decimal) Equal(other Decimal) bool {
	return d.String() == other.String()
}

func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Decimal) UnmarshalText(text []byte) error {
	parsed, err := ParseDecimal(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return d.UnmarshalText([]byte(raw))
}
