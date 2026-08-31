package review

import (
	"math"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

type ChangedEntity struct {
	Entity                *domain.Entity
	Approximate           bool
	Path                  string
	Hunks                 []Hunk
	AddedLines            []AddedLine
	AddedLinesTruncated   bool
	DeletedLines          []DeletedLine
	DeletedLinesTruncated bool
}

func MapToEntities(diffs []FileDiff, idx *query.Index) ([]ChangedEntity, []string) {
	var changed []ChangedEntity
	var unmapped []string

	for _, d := range diffs {
		if d.Status == FileDeleted {
			if strings.HasSuffix(d.Path, ".go") {
				unmapped = append(unmapped, d.Path+" (deleted — no head-side mapping)")
			}
			continue
		}

		path := d.Path
		if d.Status == FileRenamed && d.OldPath != "" {
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
					Entity:                e,
					Approximate:           false,
					Path:                  d.Path,
					Hunks:                 d.Hunks,
					AddedLines:            addedLinesForEntity(d.AddedContent, d.Hunks, e, d.Path),
					AddedLinesTruncated:   d.AddedContentTruncated,
					DeletedLines:          append([]DeletedLine(nil), d.DeletedContent...),
					DeletedLinesTruncated: d.DeletedContentTruncated,
				})
			}
			continue
		}

		// A merged entity can be declared in one file and have implementation
		// files recorded in Files. Its primary source span is not meaningful for
		// a hunk in one of those additional files, so map the hunk to the entity
		// conservatively and label it approximate instead of comparing unrelated
		// line numbers.
		for _, entity := range entities {
			if entity.Source.File == path {
				continue
			}
			changed = append(changed, ChangedEntity{
				Entity:                entity,
				Approximate:           true,
				Path:                  d.Path,
				Hunks:                 d.Hunks,
				AddedLines:            addedLinesForEntity(d.AddedContent, d.Hunks, entity, d.Path),
				AddedLinesTruncated:   d.AddedContentTruncated,
				DeletedLines:          append([]DeletedLine(nil), d.DeletedContent...),
				DeletedLinesTruncated: d.DeletedContentTruncated,
			})
		}

		sourceEntities := make([]*domain.Entity, 0, len(entities))
		for _, entity := range entities {
			if entity.Source.File == path {
				sourceEntities = append(sourceEntities, entity)
			}
		}
		sort.Slice(sourceEntities, func(i, j int) bool {
			if sourceEntities[i].Source.Line != sourceEntities[j].Source.Line {
				return sourceEntities[i].Source.Line < sourceEntities[j].Source.Line
			}
			return sourceEntities[i].ID < sourceEntities[j].ID
		})

		for i, entity := range sourceEntities {
			startLine := entity.Source.Line
			endLine := math.MaxInt32
			if entity.Source.EndLine >= startLine {
				endLine = entity.Source.EndLine
			}
			if i+1 < len(sourceEntities) {
				nextStart := sourceEntities[i+1].Source.Line - 1
				if nextStart >= startLine && nextStart < endLine {
					endLine = nextStart
				}
			}

			var overlapping []Hunk
			for _, hunk := range d.Hunks {
				hunkEnd := hunk.NewStart + hunk.NewCount - 1
				if hunkEnd < hunk.NewStart {
					hunkEnd = hunk.NewStart
				}
				if hunk.NewStart <= endLine && hunkEnd >= startLine {
					overlapping = append(overlapping, hunk)
				}
			}

			if len(overlapping) > 0 {
				changed = append(changed, ChangedEntity{
					Entity:                entity,
					Approximate:           entity.Source.EndLine == 0,
					Path:                  d.Path,
					Hunks:                 overlapping,
					AddedLines:            addedLinesForEntity(d.AddedContent, overlapping, entity, d.Path),
					AddedLinesTruncated:   d.AddedContentTruncated,
					DeletedLines:          append([]DeletedLine(nil), d.DeletedContent...),
					DeletedLinesTruncated: d.DeletedContentTruncated,
				})
			}
		}
	}

	return changed, unmapped
}

func addedLinesForEntity(lines []AddedLine, hunks []Hunk, entity *domain.Entity, path string) []AddedLine {
	var result []AddedLine
	for _, line := range lines {
		if !lineInHunks(line.Line, hunks) {
			continue
		}
		if entity != nil && entity.Source.File == path && entity.Source.EndLine >= entity.Source.Line &&
			(line.Line < entity.Source.Line || line.Line > entity.Source.EndLine) {
			continue
		}
		result = append(result, line)
	}
	return result
}

func lineInHunks(line int, hunks []Hunk) bool {
	for _, hunk := range hunks {
		end := hunk.NewStart + hunk.NewCount - 1
		if hunk.NewCount == 0 {
			end = hunk.NewStart
		}
		if line >= hunk.NewStart && line <= end {
			return true
		}
	}
	return false
}

func entitiesInFile(idx *query.Index, path string) []*domain.Entity {
	candidates := idx.EntitiesInFile(path, 200)
	entities := make([]*domain.Entity, 0, len(candidates))
	for _, entity := range candidates {
		if entity.Source.File == path {
			entities = append(entities, entity)
			continue
		}
		// Files can legitimately span a controller assembled from setup and
		// implementation files. A package entity, however, records its package
		// file set and has no source span for each file; mapping its foreign-file
		// match would create a false line-level change.
		if entity.Kind == domain.KindController || entity.Kind == domain.KindFunction || entity.Kind == domain.KindTest {
			entities = append(entities, entity)
		}
	}
	return entities
}
