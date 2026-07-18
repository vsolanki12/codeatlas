package parser

import "github.com/vsolanki12/hypershift-atlas/internal/domain"

type Parser interface {
	Parse(file domain.File) ([]domain.Entity, error)
}
