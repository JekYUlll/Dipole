package app

import coreapplication "github.com/JekYUlll/Dipole/internal/services/core/application"

// NewLocalCoreCapability keeps the embedded composition API stable while the
// Core capability implementation lives under its service boundary.
func NewLocalCoreCapability(repos *Repositories) *coreapplication.LocalCoreCapability {
	return coreapplication.New(coreapplication.Dependencies{
		Users: repos.Users, Contacts: repos.Contacts, Groups: repos.Groups,
		Files: repos.Files, Conversations: repos.Conversations,
	})
}
