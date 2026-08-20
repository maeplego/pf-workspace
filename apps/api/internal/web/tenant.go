package web

import (
	"context"

	"github.com/portfolio/pf-workspace/api/internal/service"
)

type tenantSvcKey struct{}

func withTenantSvc(ctx context.Context, svc *service.Service) context.Context {
	return context.WithValue(ctx, tenantSvcKey{}, svc)
}

func (s *Server) ts(ctx context.Context) *service.Service {
	if svc, ok := ctx.Value(tenantSvcKey{}).(*service.Service); ok {
		return svc
	}
	return s.svc
}
