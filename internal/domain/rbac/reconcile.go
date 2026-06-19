package rbac

import (
	"context"
	"fmt"
	"log"
	"strings"
)

func ReconcileRoles(ctx context.Context, repo Repository, profileID string, userID string, positions string) (addedNames, removedNames []string, err error) {
	currentRoles, err := repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user roles: %w", err)
	}

	protected := map[string]bool{
		"parent":     true,
		"admin":      true,
		"Scouts BSA": true,
	}

	currentRoleNames := make(map[string]bool)
	for _, role := range currentRoles {
		if !protected[role.Name] {
			currentRoleNames[role.Name] = true
		}
	}

	targetPositions := make(map[string]bool)
	if positions != "" {
		for _, pos := range strings.Split(positions, ", ") {
			targetPositions[pos] = true
		}
	}

	for pos := range targetPositions {
		if !currentRoleNames[pos] {
			role, err := repo.GetRoleByName(ctx, pos)
			if err != nil {
				role = &Role{Name: pos}
				if err := repo.CreateRole(ctx, role); err != nil {
					return nil, nil, fmt.Errorf("create role %q: %w", pos, err)
				}
			}
			if err := repo.AssignRoleToUser(ctx, userID, role.ID); err != nil {
				return nil, nil, fmt.Errorf("assign role %q: %w", pos, err)
			}
			log.Printf("[rbac] ROLE ADDED profileId=%s role=%s", profileID, pos)
			addedNames = append(addedNames, pos)
		}
	}

	for roleName := range currentRoleNames {
		if !targetPositions[roleName] {
			role, err := repo.GetRoleByName(ctx, roleName)
			if err != nil {
				continue
			}
			if err := repo.RemoveRoleFromUser(ctx, userID, role.ID); err != nil {
				return nil, nil, fmt.Errorf("remove role %q: %w", roleName, err)
			}
			log.Printf("[rbac] ROLE REMOVED profileId=%s role=%s", profileID, roleName)
			removedNames = append(removedNames, roleName)
		}
	}

	return addedNames, removedNames, nil
}
