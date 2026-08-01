package parser

import "github.com/vsolanki12/codeatlas/internal/domain"

type Parser interface {
	Parse(file domain.File) ([]domain.Entity, error)
}
