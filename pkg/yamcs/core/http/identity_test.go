package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdentityReplacesCredentials(t *testing.T) {
	var auth, idt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, idt = r.Header.Get("Authorization"), r.Header.Get("X-Id-Token")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()
	m := &HTTPManager{Client: srv.Client(), Credentials: &BasicAuthCredentials{Username: "grafana", Password: "p"}}

	if _, err := m.SendRequest(context.Background(), "GET", srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if auth == "" || idt != "" {
		t.Errorf("host's call: Authorization %q, X-Id-Token %q", auth, idt)
	}
	ctx := WithIdentity(context.Background(), map[string]string{"X-Id-Token": "tok"})
	if _, err := m.SendRequest(ctx, "POST", srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if auth != "" || idt != "tok" {
		t.Errorf("user's call: Authorization %q, X-Id-Token %q", auth, idt)
	}
}
