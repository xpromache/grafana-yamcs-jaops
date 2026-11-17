package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/jaops-space/grafana-yamcs-jaops/api/yamcs/protobuf/instances"
	"github.com/jaops-space/grafana-yamcs-jaops/pkg/config"
	"github.com/jaops-space/grafana-yamcs-jaops/pkg/source"
)

func TestActionRoutesDoNotRestrictViewerRole(t *testing.T) {
	router := mux.NewRouter()
	(&Datasource{multiplexer: &source.Multiplexer{Endpoints: map[string]*source.YamcsEndpoint{}}}).registerRoutes(router)
	actionPaths := []string{
		"/endpoint/e1/command/issue",
		"/endpoint/e1/alarm/acknowledge",
		"/endpoint/e1/alarm/clear",
		"/endpoint/e1/alarm/shelve",
		"/endpoint/e1/alarm/unshelve",
		"/endpoint/e1/links/link/enable",
		"/endpoint/e1/links/link/disable",
		"/endpoint/e1/links/link/reset",
		"/endpoint/e1/links/link/action/action",
	}

	for _, path := range actionPaths {
		for _, role := range []string{"", "Viewer"} {
			t.Run(path+"/"+role, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				if role != "" {
					req = req.WithContext(backend.WithUser(context.Background(), &backend.User{Role: role}))
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code == http.StatusForbidden {
					t.Fatalf("%s role was unexpectedly denied", role)
				}
			})
		}
	}
}

func TestExecuteCommandRejectsNullRequestBody(t *testing.T) {
	router := mux.NewRouter()
	datasource := &Datasource{
		multiplexer: &source.Multiplexer{
			Endpoints: map[string]*source.YamcsEndpoint{},
		},
	}
	datasource.registerRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/endpoint/e1/command/issue", strings.NewReader("null"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestListEndpoints(t *testing.T) {
	name, running := "fm1", instances.YamcsInstance_RUNNING
	host := &source.YamcsHost{
		Configuration: &config.YamcsHostConfiguration{ID: "yamcs-a"},
		Instances: map[string]*source.YamcsHostInstance{
			"fm1": {Instance: &instances.YamcsInstance{Name: &name, State: &running, Labels: map[string]string{"mdb": "pussim"}}},
		},
	}
	router := mux.NewRouter()
	(&Datasource{multiplexer: &source.Multiplexer{
		Hosts: map[string]*source.YamcsHost{"yamcs-a": host},
		Endpoints: map[string]*source.YamcsEndpoint{
			"sat-1": {Host: host, Discovered: true, Configuration: &config.YamcsEndpointConfiguration{ID: "sat-1", Host: "yamcs-a", Instance: "fm1"}},
		},
	}}).registerRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list/endpoints", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]struct {
		Instance   string            `json:"instance"`
		Host       string            `json:"host"`
		Labels     map[string]string `json:"labels"`
		Discovered bool              `json:"discovered"`
		Online     bool              `json:"online"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	ep, ok := got["sat-1"]
	if !ok || ep.Instance != "fm1" || ep.Host != "yamcs-a" || ep.Labels["mdb"] != "pussim" || !ep.Discovered || ep.Online {
		t.Errorf("got %+v", got)
	}
}
