// Package security provides domain types for the security module.
package security

// Role represents a user role in the system.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleOperator  Role = "operator"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
	RoleService   Role = "service"
)

// IsValid returns true if the role is valid.
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleDeveloper, RoleViewer, RoleService:
		return true
	default:
		return false
	}
}

// Permission represents a specific permission.
type Permission string

const (
	// Swarm permissions
	PermSwarmCreate Permission = "swarm.create"
	PermSwarmRead   Permission = "swarm.read"
	PermSwarmUpdate Permission = "swarm.update"
	PermSwarmDelete Permission = "swarm.delete"
	PermSwarmScale  Permission = "swarm.scale"

	// Agent permissions
	PermAgentSpawn     Permission = "agent.spawn"
	PermAgentRead      Permission = "agent.read"
	PermAgentTerminate Permission = "agent.terminate"

	// Task permissions
	PermTaskCreate Permission = "task.create"
	PermTaskRead   Permission = "task.read"
	PermTaskCancel Permission = "task.cancel"

	// Metrics permissions
	PermMetricsRead Permission = "metrics.read"

	// System permissions
	PermSystemAdmin  Permission = "system.admin"
	PermSystemConfig Permission = "system.config"

	// API permissions
	PermAPIAccess Permission = "api.access"
)

// AllPermissions returns all available permissions.
func AllPermissions() []Permission {
	return []Permission{
		PermSwarmCreate, PermSwarmRead, PermSwarmUpdate, PermSwarmDelete, PermSwarmScale,
		PermAgentSpawn, PermAgentRead, PermAgentTerminate,
		PermTaskCreate, PermTaskRead, PermTaskCancel,
		PermMetricsRead,
		PermSystemAdmin, PermSystemConfig,
		PermAPIAccess,
	}
}

// PermissionSet represents a set of permissions.
type PermissionSet map[Permission]bool

// NewPermissionSet creates a new permission set.
func NewPermissionSet(permissions ...Permission) PermissionSet {
	ps := make(PermissionSet)
	for _, p := range permissions {
		ps[p] = true
	}
	return ps
}

// Has returns true if the set contains the permission.
func (ps PermissionSet) Has(permission Permission) bool {
	return ps[permission]
}

// Add adds a permission to the set.
func (ps PermissionSet) Add(permission Permission) {
	ps[permission] = true
}

// Remove removes a permission from the set.
func (ps PermissionSet) Remove(permission Permission) {
	delete(ps, permission)
}

// Merge merges another permission set into this one.
func (ps PermissionSet) Merge(other PermissionSet) {
	for p := range other {
		ps[p] = true
	}
}

// ToSlice returns the permissions as a slice.
func (ps PermissionSet) ToSlice() []Permission {
	result := make([]Permission, 0, len(ps))
	for p := range ps {
		result = append(result, p)
	}
	return result
}

// RolePermissions maps roles to their default permissions.
var RolePermissions = map[Role]PermissionSet{
	RoleAdmin: NewPermissionSet(
		PermSwarmCreate, PermSwarmRead, PermSwarmUpdate, PermSwarmDelete, PermSwarmScale,
		PermAgentSpawn, PermAgentRead, PermAgentTerminate,
		PermTaskCreate, PermTaskRead, PermTaskCancel,
		PermMetricsRead,
		PermSystemAdmin, PermSystemConfig,
		PermAPIAccess,
	),
	RoleOperator: NewPermissionSet(
		PermSwarmRead, PermSwarmUpdate, PermSwarmScale,
		PermAgentSpawn, PermAgentRead, PermAgentTerminate,
		PermTaskCreate, PermTaskRead, PermTaskCancel,
		PermMetricsRead,
		PermAPIAccess,
	),
	RoleDeveloper: NewPermissionSet(
		PermSwarmRead,
		PermAgentSpawn, PermAgentRead,
		PermTaskCreate, PermTaskRead, PermTaskCancel,
		PermMetricsRead,
		PermAPIAccess,
	),
	RoleViewer: NewPermissionSet(
		PermSwarmRead,
		PermAgentRead,
		PermTaskRead,
		PermMetricsRead,
		PermAPIAccess,
	),
	RoleService: NewPermissionSet(
		PermSwarmRead,
		PermAgentSpawn, PermAgentRead,
		PermTaskCreate, PermTaskRead,
		PermMetricsRead,
		PermAPIAccess,
	),
}

// GetRolePermissions returns the default permissions for a role.
func GetRolePermissions(role Role) PermissionSet {
	if perms, ok := RolePermissions[role]; ok {
		// Return a copy to prevent mutation
		result := make(PermissionSet)
		for p := range perms {
			result[p] = true
		}
		return result
	}
	return make(PermissionSet)
}

// CanPerform checks if a role can perform a specific action.
func CanPerform(role Role, permission Permission) bool {
	perms := GetRolePermissions(role)
	return perms.Has(permission)
}
