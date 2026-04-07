package services

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"fmt"
	"log"
	"strings"
)

// LinkerService handles the resolution of symbol references after indexing.
type LinkerService struct {
	// No state for now
}

// NewLinkerService creates a new instance of LinkerService.
func NewLinkerService() *LinkerService {
	return &LinkerService{}
}

// ResolveUsages iterates through all unresolved symbol usages in a project and attempts to link them.
func (s *LinkerService) ResolveUsages(projectID string, vs *store.VectorStore) error {
	log.Printf("Starting symbol resolution (Linking) for project %s", projectID)

	usages, err := vs.ListUnresolvedUsages()
	if err != nil {
		return fmt.Errorf("failed to list unresolved usages: %w", err)
	}

	if len(usages) == 0 {
		log.Printf("No unresolved usages found for project %s", projectID)
		return nil
	}

	resolvedCount := 0
	virtualCount := 0
	updates := make(map[int64]string)

	for _, u := range usages {
		targetID, isVirtual, err := s.resolveTarget(u, vs)
		if err != nil {
			log.Printf("Warning: failed to resolve usage %d: %v", u.ID, err)
			continue
		}

		if targetID != "" {
			updates[u.ID] = targetID
			if isVirtual {
				virtualCount++
			}
		}
	}

	if len(updates) > 0 {
		if err := vs.UpdateSymbolUsageTargets(updates); err != nil {
			log.Printf("Error bulk updating usages: %v", err)
		} else {
			resolvedCount = len(updates)
		}
	}

	log.Printf("Linking completed: %d resolved (%d virtual) out of %d total", resolvedCount, virtualCount, len(usages))

	if purged, err := vs.PurgeOrphanedVirtualFiles(); err == nil && purged > 0 {
		log.Printf("Cleanup: Purged %d unused external library references", purged)
	}

	return nil
}

// ResolveFileUsages attempts to link all unresolved symbols within a specific file.
func (s *LinkerService) ResolveFileUsages(projectID string, filePath string, vs *store.VectorStore) error {
	usages, err := vs.ListUnresolvedUsagesForFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to list unresolved usages for file %s: %w", filePath, err)
	}

	if len(usages) == 0 {
		return nil
	}

	resolvedCount := 0
	updates := make(map[int64]string)

	for _, u := range usages {
		targetID, _, err := s.resolveTarget(u, vs)
		if err != nil {
			continue
		}

		if targetID != "" {
			updates[u.ID] = targetID
		}
	}

	if len(updates) > 0 {
		if err := vs.UpdateSymbolUsageTargets(updates); err == nil {
			resolvedCount = len(updates)
		}
	}

	if resolvedCount > 0 {
		log.Printf("Incremental linking for %s: %d symbols resolved", filePath, resolvedCount)
	}

	return nil
}

// resolveTarget tries to find a matching node for a usage record.
func (s *LinkerService) resolveTarget(u *models.SymbolUsage, vs *store.VectorStore) (string, bool, error) {
	// 1. Search for internal symbols by name
	candidates, err := vs.FindSymbolNodesByName(u.RawTargetName)
	if err != nil {
		return "", false, err
	}

	if len(candidates) == 1 {
		return candidates[0].ID, false, nil
	}

	// 2. Ambiguity resolution using context (Heuristic)
	if len(candidates) > 1 {
		// Prefer internal symbols over external/virtual ones
		for _, c := range candidates {
			if c.Kind != "external" {
				return c.ID, false, nil
			}
		}
		// If all are external or we still have ambiguity, pick the first
		return candidates[0].ID, false, nil
	}

	// 3. Fallback to Virtual Symbol (External Dependency)
	// Example path: @external/go/fmt
	virtualPath := "@external/unknown"
	if u.RawTargetContext != "" {
		virtualPath = "@external/" + strings.ToLower(u.RawTargetContext)
	}

	// Create a stable ID for the virtual symbol using the unified slug format
	// Virtual symbols are treated as being at L0-0 (unknown location)
	virtualID := utils.GenerateSymbolID(virtualPath, 0, 0, u.RawTargetName, 1)

	virtualNode := &models.OutlineNode{
		ID:        virtualID,
		Name:      u.RawTargetName,
		Kind:      "external",
		StartLine: 0,
		EndLine:   0,
	}

	if err := vs.InsertVirtualSymbol(virtualPath, virtualNode); err != nil {
		return "", false, fmt.Errorf("failed to create virtual symbol '%s' (target from source code): %w", 
			u.RawTargetName, err)
	}

	return virtualID, true, nil
}
