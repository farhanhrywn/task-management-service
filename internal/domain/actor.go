package domain

import "github.com/google/uuid"

// Actor is the authenticated caller of a use case.
type Actor struct {
	UserID uuid.UUID
	Role   Role
}

func (a Actor) IsSuperAdmin() bool { return a.Role == RoleSuperAdmin }

// CanAccessTask reports whether the actor may read or modify t: the owner and
// the super admin can; everyone else (including the assignee) cannot.
func (a Actor) CanAccessTask(t *Task) bool {
	return a.IsSuperAdmin() || t.UserID == a.UserID
}
