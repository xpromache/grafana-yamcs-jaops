package plugin

import (
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/jaops-space/grafana-yamcs-jaops/pkg/config"
	corehttp "github.com/jaops-space/grafana-yamcs-jaops/pkg/yamcs/core/http"
)

// roleRank orders Grafana's organisation roles.
var roleRank = map[string]int{"viewer": 1, "editor": 2, "admin": 3}

// validRole reports whether r is empty or a Grafana role.
func validRole(r string) bool {
	_, ok := roleRank[strings.ToLower(r)]
	return r == "" || ok
}

// action wraps a handler that changes something in Yamcs (a command, an
// alarm or link action): refused when actions are disabled or the user's
// Grafana role is below the minimum, and sent with the user's ID token when
// actions are made as the user.
func (d *Datasource) action(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		cfg := d.actions
		user := backend.UserFromContext(req.Context())
		login := ""
		if user != nil {
			login = user.Login
		}
		if cfg.Disabled {
			http.Error(w, "actions are disabled on this datasource", http.StatusForbidden)
			return
		}
		if cfg.MinRole != "" {
			role := ""
			if user != nil {
				role = user.Role
			}
			if roleRank[strings.ToLower(role)] < roleRank[strings.ToLower(cfg.MinRole)] {
				backend.Logger.Warn("Action refused: role too low", "user", login, "role", role, "minRole", cfg.MinRole, "path", req.URL.Path)
				http.Error(w, "your Grafana role may not act on Yamcs (needs "+cfg.MinRole+")", http.StatusForbidden)
				return
			}
		}
		if cfg.AsUser {
			token := req.Header.Get(backend.OAuthIdentityIDTokenHeaderName)
			if token == "" {
				backend.Logger.Warn("Action refused: no ID token forwarded", "user", login, "path", req.URL.Path)
				http.Error(w, "no identity token: log in to Grafana with single sign-on, and enable Forward OAuth Identity on the datasource", http.StatusUnauthorized)
				return
			}
			req = req.WithContext(corehttp.WithIdentity(req.Context(), map[string]string{cfg.TokenHeader(): token}))
		}
		backend.Logger.Info("Yamcs action", "user", login, "path", req.URL.Path)
		next(w, req)
	}
}

func actionsOf(cfg *config.YamcsPluginConfiguration) config.ActionsConfiguration {
	if cfg == nil {
		return config.ActionsConfiguration{}
	}
	return cfg.Actions
}
