package review

import (
	"math"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

type ChangedEntity struct {
	Entity      *domain.Entity
	Approximate bool
	Hunks       []Hunk
}

func MapToEntities(diffs []FileDiff, idx *query.Index) ([]ChangedEntity, []string) {
	var changed []ChangedEntity
	var unmapped []string

	for _, d := range diffs {
		if d.Status == FileDeleted {
			continue
		}

		path := d.OldPath
		if d.Status == FileAdded {
			path = d.Path
		}

		entities := entitiesInFile(idx, path)
		if len(entities) == 0 {
			if strings.HasSuffix(d.Path, ".go") {
				unmapped = append(unmapped, d.Path)
			}
			continue
		}

		if d.Status == FileAdded {
			for _, e := range entities {
				changed = append(changed, ChangedEntity{
					Entity:      e,
					Approximate: false,
					Hunks:       d.Hunks,
				})
			}
			continue
		}

		sort.Slice(entities, func(i, j int) bool {
			return entities[i].Source.Line < entities[j].Source.Line
		})

		for i, entity := range entities {
			startLine := entity.Source.Line
			endLine := math.MaxInt32
			if i+1 < len(entities) {
				endLine = entities[i+1].Source.Line - 1
				if endLine < startLine {
					endLine = startLine
				}
			}

			var overlapping []Hunk
			for _, hunk := range d.Hunks {
				hunkEnd := hunk.OldStart + hunk.OldCount - 1
				if hunkEnd < hunk.OldStart {
					hunkEnd = hunk.OldStart
				}
				if hunk.OldStart <= endLine && hunkEnd >= startLine {
					overlapping = append(overlapping, hunk)
				}
			}

			if len(overlapping) > 0 {
				changed = append(changed, ChangedEntity{
					Entity:      entity,
					Approximate: true,
					Hunks:       overlapping,
				})
			}
		}
	}

	return changed, unmapped
}

func entitiesInFile(idx *query.Index, path string) []*domain.Entity {
	all := idx.Where(path, 200)
	var exact []*domain.Entity
	for _, e := range all {
		if e.Source.File == path {
			exact = append(exact, e)
		}
	}
	return exact
}
