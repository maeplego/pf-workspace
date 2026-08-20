package service

import "github.com/portfolio/pf-workspace/api/internal/store"

// ConfigureTenant wires Postgres RLS scoping. Memory-only tests omit this.
func (s *Service) ConfigureTenant(scope func(orgID string) store.Store, unscoped func() store.Store) {
	s.tenantScope = scope
	s.unscopedStore = unscoped
}

// ForOrg returns a shallow copy using SET LOCAL app.tenant_id via scoped store.
func (s *Service) ForOrg(orgID string) *Service {
	if s.tenantScope == nil {
		return s
	}
	c := *s
	c.store = s.tenantScope(orgID)
	return &c
}

// Unscoped bypasses tenant RLS (invite token lookup, internal collab, migration-style reads).
func (s *Service) Unscoped() *Service {
	if s.unscopedStore == nil {
		return s
	}
	c := *s
	c.store = s.unscopedStore()
	return &c
}
