package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

type userAPI struct {
	t  *testing.T
	r  *gin.Engine
	st store.Store
}

func newUserAPI(t *testing.T) *userAPI {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := auth.GenerateDevKey()
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemory()
	guard := &auth.Guard{Mode: auth.ModeEnforce, Issuer: auth.NewIssuer(key, 0), Public: []string{"/api/v1/auth/login"}}
	guard.UserState = func(ctx context.Context, id string) (bool, int, error) {
		u, err := st.GetUser(ctx, id)
		return err == nil && u.Active, u.TokenVersion, nil
	}
	env := map[string]string{"AUTH_BOOTSTRAP_EMAIL": "root@aipods.ar", "AUTH_BOOTSTRAP_PASSWORD": "root-password-123"}
	if err := bootstrapAdmin(context.Background(), st, func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(guard.Authenticate())
	a := auditor{st: st, guard: guard}
	registerAuthRoutes(r, st, guard, a)
	registerUserRoutes(r, st, guard, a)
	return &userAPI{t: t, r: r, st: st}
}

func (u *userAPI) call(method, path, token string, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	u.r.ServeHTTP(w, req)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (u *userAPI) login(email, password string) string {
	code, out := u.call("POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": password})
	if code != 200 {
		u.t.Fatalf("login %s: %d %v", email, code, out)
	}
	return out["access_token"].(string)
}

func (u *userAPI) create(token, tenantID, email string, roles ...string) string {
	code, out := u.call("POST", "/api/v1/admin/users", token, map[string]any{
		"tenant_id": tenantID, "email": email, "password": "initial-pass-123", "roles": roles})
	if code != 201 {
		u.t.Fatalf("create %s: %d %v", email, code, out)
	}
	return out["user"].(map[string]any)["id"].(string)
}

func TestChangeOwnPassword(t *testing.T) {
	api := newUserAPI(t)
	tok := api.login("root@aipods.ar", "root-password-123")

	if code, _ := api.call("POST", "/api/v1/auth/password", tok, map[string]string{"current_password": "wrong-password-1", "new_password": "brand-new-pass-456"}); code != 400 {
		t.Errorf("wrong current password: want 400 (not 401, so the session survives), got %d", code)
	}
	if code, _ := api.call("POST", "/api/v1/auth/password", tok, map[string]string{"current_password": "root-password-123", "new_password": "short"}); code != 400 {
		t.Errorf("too short: want 400, got %d", code)
	}
	if code, _ := api.call("POST", "/api/v1/auth/password", "", map[string]string{"current_password": "x", "new_password": "y"}); code != 401 {
		t.Errorf("no token: want 401, got %d", code)
	}
	other := api.login("root@aipods.ar", "root-password-123") // a second open session
	code, out := api.call("POST", "/api/v1/auth/password", tok, map[string]string{"current_password": "root-password-123", "new_password": "brand-new-pass-456"})
	if code != 200 {
		t.Fatalf("change: %d %v", code, out)
	}
	fresh, _ := out["access_token"].(string)
	if fresh == "" {
		t.Fatal("the change must return a new token so this session stays open")
	}
	for name, old := range map[string]string{"same session": tok, "other session": other} {
		if code, _ := api.call("GET", "/api/v1/admin/users", old, nil); code != 401 {
			t.Errorf("%s token from before the change must get 401, got %d", name, code)
		}
	}
	if code, _ := api.call("GET", "/api/v1/admin/users", fresh, nil); code != 200 {
		t.Errorf("new token must work, got %d", code)
	}
	if code, _ := api.call("POST", "/api/v1/auth/login", "", map[string]string{"email": "root@aipods.ar", "password": "root-password-123"}); code != 401 {
		t.Errorf("old password must stop working, got %d", code)
	}
	api.login("root@aipods.ar", "brand-new-pass-456")
}

func TestUserAdministration(t *testing.T) {
	api := newUserAPI(t)
	root := api.login("root@aipods.ar", "root-password-123")
	adminID := api.create(root, "tenant_acme", "jefa@acme.test", auth.RoleTenantAdmin)
	anaID := api.create(root, "tenant_acme", "ana@acme.test", auth.RoleCustomer)
	otroID := api.create(root, "tenant_otro", "luis@otro.test", auth.RoleCustomer)
	auditorID := api.create(root, "tenant_acme", "audit@acme.test", auth.RoleAuditor)
	_ = adminID

	// Listing
	if code, out := api.call("GET", "/api/v1/admin/users", root, nil); code != 200 || len(out["users"].([]any)) != 5 {
		t.Fatalf("platform_admin lists everyone: %d %v", code, out)
	}
	jefa := api.login("jefa@acme.test", "initial-pass-123")
	code, out := api.call("GET", "/api/v1/admin/users?tenant_id=tenant_otro", jefa, nil)
	if code != 403 {
		t.Errorf("SECURITY: tenant_admin listed another tenant: %d %v", code, out)
	}
	code, out = api.call("GET", "/api/v1/admin/users", jefa, nil)
	if code != 200 || len(out["users"].([]any)) != 3 {
		t.Errorf("tenant_admin sees its 3 users: %d %v", code, out)
	}
	for _, raw := range out["users"].([]any) {
		if _, leaked := raw.(map[string]any)["password_hash"]; leaked {
			t.Fatal("SECURITY: password hash in the listing")
		}
	}
	ana := api.login("ana@acme.test", "initial-pass-123")
	if code, _ := api.call("GET", "/api/v1/admin/users", ana, nil); code != 403 {
		t.Errorf("customer cannot list users, got %d", code)
	}

	// tenant_admin limits
	if code, _ := api.call("PATCH", "/api/v1/admin/users/"+otroID, jefa, map[string]bool{"active": false}); code != 404 {
		t.Errorf("SECURITY: other tenant's user must look missing (404), got %d", code)
	}
	if code, _ := api.call("PATCH", "/api/v1/admin/users/"+auditorID, jefa, map[string]bool{"active": false}); code != 403 {
		t.Errorf("tenant_admin cannot touch staff, got %d", code)
	}
	if code, _ := api.call("PATCH", "/api/v1/admin/users/"+adminID, jefa, map[string]bool{"active": false}); code != 400 {
		t.Errorf("cannot deactivate yourself, got %d", code)
	}

	// Reset password, then deactivate: the open session dies at once.
	if code, out := api.call("POST", "/api/v1/admin/users/"+anaID+"/password", jefa, map[string]string{"new_password": "reset-by-admin-789"}); code != 200 {
		t.Fatalf("reset: %d %v", code, out)
	}
	if code, _ := api.call("POST", "/api/v1/auth/password", ana, map[string]string{"current_password": "x", "new_password": "y"}); code != 401 {
		t.Errorf("an admin reset must end the user's open sessions, got %d", code)
	}
	ana = api.login("ana@acme.test", "reset-by-admin-789")
	if code, _ := api.call("GET", "/api/v1/auth/me", ana, nil); code != 200 {
		t.Fatalf("ana should be in, got %d", code)
	}
	if code, out := api.call("PATCH", "/api/v1/admin/users/"+anaID, jefa, map[string]bool{"active": false}); code != 200 || out["user"].(map[string]any)["active"] != false {
		t.Fatalf("deactivate: %d %v", code, out)
	}
	if code, _ := api.call("GET", "/api/v1/auth/me", ana, nil); code != 401 {
		t.Errorf("deactivated user's token must stop working, got %d", code)
	}
	if code, _ := api.call("POST", "/api/v1/auth/login", "", map[string]string{"email": "ana@acme.test", "password": "reset-by-admin-789"}); code != 401 {
		t.Errorf("deactivated user cannot log in, got %d", code)
	}
	if code, _ := api.call("PATCH", "/api/v1/admin/users/"+anaID, jefa, map[string]bool{"active": true}); code != 200 {
		t.Errorf("reactivate: got %d", code)
	}
	api.login("ana@acme.test", "reset-by-admin-789")

	// Audit trail
	events, _ := api.st.ListAudit(context.Background(), "tenant_acme", 50)
	seen := map[string]bool{}
	for _, e := range events {
		seen[e.Action] = true
	}
	for _, want := range []string{"user.password_reset", "user.deactivate", "user.activate"} {
		if !seen[want] {
			t.Errorf("missing audit event %s", want)
		}
	}
}

func TestTenantAdminManagesRoles(t *testing.T) {
	api := newUserAPI(t)
	root := api.login("root@aipods.ar", "root-password-123")
	api.create(root, "tenant_acme", "jefa@acme.test", auth.RoleTenantAdmin)
	anaID := api.create(root, "tenant_acme", "ana@acme.test", auth.RoleCustomer)
	otroID := api.create(root, "tenant_otro", "luis@otro.test", auth.RoleCustomer)
	jefa := api.login("jefa@acme.test", "initial-pass-123")
	ana := api.login("ana@acme.test", "initial-pass-123")
	path := "/api/v1/admin/users/" + anaID + "/roles"

	if code, _ := api.call("PATCH", path, jefa, map[string][]string{"roles": {auth.RoleAuditor}}); code != 403 {
		t.Errorf("tenant_admin cannot grant auditor, got %d", code)
	}
	if code, _ := api.call("PATCH", path, jefa, map[string][]string{"roles": {}}); code != 400 {
		t.Errorf("empty roles must be rejected, got %d", code)
	}
	if code, _ := api.call("PATCH", path, jefa, map[string][]string{"roles": {"owner"}}); code != 400 {
		t.Errorf("unknown role must be rejected, got %d", code)
	}
	if code, _ := api.call("PATCH", "/api/v1/admin/users/"+otroID+"/roles", jefa, map[string][]string{"roles": {auth.RoleTenantAdmin}}); code != 404 {
		t.Errorf("other tenant's user must look missing, got %d", code)
	}
	if code, _ := api.call("PATCH", path, ana, map[string][]string{"roles": {auth.RoleTenantAdmin}}); code != 403 {
		t.Errorf("a customer cannot change roles, got %d", code)
	}
	code, out := api.call("PATCH", path, jefa, map[string][]string{"roles": {auth.RoleTenantAdmin}})
	if code != 200 || out["user"].(map[string]any)["roles"].([]any)[0] != auth.RoleTenantAdmin {
		t.Fatalf("promote ana: %d %v", code, out)
	}
	if code, _ := api.call("GET", "/api/v1/auth/me", ana, nil); code != 401 {
		t.Errorf("token with old roles must stop working, got %d", code)
	}
	ana = api.login("ana@acme.test", "initial-pass-123")
	if code, _ := api.call("GET", "/api/v1/admin/users", ana, nil); code != 200 {
		t.Errorf("promoted ana can list her team, got %d", code)
	}

	// Audit log scoped to the tenant_admin's own tenant.
	code, out = api.call("GET", "/api/v1/admin/audit/log?tenant_id=tenant_otro", jefa, nil)
	if code != 403 {
		t.Errorf("tenant_admin cannot read another tenant's audit, got %d %v", code, out)
	}
	code, out = api.call("GET", "/api/v1/admin/audit/log", jefa, nil)
	if code != 200 {
		t.Fatalf("tenant_admin reads its audit: %d %v", code, out)
	}
	found := false
	for _, raw := range out["events"].([]any) {
		e := raw.(map[string]any)
		if e["tenant_id"] != "tenant_acme" {
			t.Errorf("leaked event from %v", e["tenant_id"])
		}
		if e["action"] == "user.roles" {
			found = true
		}
	}
	if !found {
		t.Error("missing user.roles audit event")
	}
}
