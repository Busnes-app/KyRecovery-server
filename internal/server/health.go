package server

import (
	"context"
	"net/http"

	"github.com/Busnes-app/ky-primitives/health"
	"github.com/Busnes-app/ky-primitives/logging"
)

var auditUnavailable = health.DeclareReason("append_disabled")

func healthHandler(s *Server, lg *logging.Logger) http.Handler {
	return health.Handler("kyrecovery", lg,
		health.Check{Name: "database", Run: s.db.PingContext},
		health.Check{Name: "audit", Run: func(context.Context) error {
			if s.ledger.Healthy() != nil {
				return health.Degrade(auditUnavailable)
			}
			return nil
		}},
	)
}
