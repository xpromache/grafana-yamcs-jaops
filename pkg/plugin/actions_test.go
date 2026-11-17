package plugin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/jaops-space/grafana-yamcs-jaops/pkg/config"
	corehttp "github.com/jaops-space/grafana-yamcs-jaops/pkg/yamcs/core/http"
)

func TestAction(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.ActionsConfiguration
		role    string
		token   string
		want    int
		wantHdr string
	}{
		{"open", config.ActionsConfiguration{}, "Viewer", "", 200, ""},
		{"disabled", config.ActionsConfiguration{Disabled: true}, "Admin", "", 403, ""},
		{"viewer below editor", config.ActionsConfiguration{MinRole: "Editor"}, "Viewer", "", 403, ""},
		{"editor", config.ActionsConfiguration{MinRole: "Editor"}, "Editor", "", 200, ""},
		{"admin above editor", config.ActionsConfiguration{MinRole: "editor"}, "Admin", "", 200, ""},
		{"as user without token", config.ActionsConfiguration{AsUser: true}, "Editor", "", 401, ""},
		{"as user", config.ActionsConfiguration{AsUser: true}, "Editor", "idt", 200, "idt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := &Datasource{actions: c.cfg}
			var got string
			h := d.action(func(w http.ResponseWriter, r *http.Request) {
				got = corehttp.IdentityFrom(r.Context())[c.cfg.TokenHeader()]
			})
			req := httptest.NewRequest("POST", "/endpoint/sat-1/command/issue", nil)
			if c.token != "" {
				req.Header.Set(backend.OAuthIdentityIDTokenHeaderName, c.token)
			}
			ctx := backend.WithUser(req.Context(), &backend.User{Login: "alice", Role: c.role})
			rec := httptest.NewRecorder()
			h(rec, req.WithContext(ctx))
			if rec.Code != c.want {
				t.Errorf("status %d, want %d", rec.Code, c.want)
			}
			if got != c.wantHdr {
				t.Errorf("identity header %q, want %q", got, c.wantHdr)
			}
		})
	}
}
