package validator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s é obrigatório", field)
	}
	return nil
}

func MaxLen(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return fmt.Errorf("%s deve ter no máximo %d caracteres", field, max)
	}
	return nil
}

func Slug(value string) error {
	if !slugRegex.MatchString(value) {
		return fmt.Errorf("slug inválido: use apenas letras minúsculas, números e hífens")
	}
	return nil
}

func Email(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, "@") {
		return fmt.Errorf("e-mail inválido")
	}
	return nil
}

func OneOf(field, value string, allowed ...string) error {
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("%s deve ser um dos valores: %s", field, strings.Join(allowed, ", "))
}
