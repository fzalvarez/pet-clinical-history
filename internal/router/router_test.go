package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pet-clinical-history/internal/domain/accessgrants"
	"pet-clinical-history/internal/router"
)

func TestHTTP_EndToEnd_DelegationScopes(t *testing.T) {
	ts := httptest.NewServer(router.NewRouter(router.Options{AuthVerifier: nil}))
	defer ts.Close()

	ownerID := "owner-1"
	delegateID := "delegate-1"

	// 1) Owner crea mascota
	petID := createPet(t, ts.URL, ownerID, map[string]any{
		"name":    "Milo",
		"species": "dog",
		"breed":   "mixed",
		"sex":     "male",
		"notes":   "test",
	})

	// 2) Delegado NO puede ver perfil aún
	{
		st, _ := doReq(t, ts.URL, "GET", "/pets/"+petID, delegateID, nil)
		if st != http.StatusForbidden {
			t.Fatalf("expected 403 before grant, got %d", st)
		}
	}

	// 3) Owner invita delegado con scopes necesarios
	grantID := inviteGrant(t, ts.URL, ownerID, petID, delegateID, []string{
		string(accessgrants.ScopePetRead),
		string(accessgrants.ScopePetEditProfile),
		string(accessgrants.ScopeEventsRead),
		string(accessgrants.ScopeEventsCreate),
		string(accessgrants.ScopeEventsVoid),
	})

	// 4) Delegado ve su invitación
	{
		st, body := doReq(t, ts.URL, "GET", "/me/grants", delegateID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 listing my grants, got %d body=%s", st, string(body))
		}
	}

	// 5) Delegado acepta
	{
		st, body := doReq(t, ts.URL, "POST", "/grants/"+grantID+"/accept", delegateID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 accept grant, got %d body=%s", st, string(body))
		}
	}

	// 6) Delegado ya puede ver perfil
	{
		st, body := doReq(t, ts.URL, "GET", "/pets/"+petID, delegateID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 get pet by delegate, got %d body=%s", st, string(body))
		}
	}

	// 7) Delegado puede editar perfil (PATCH)
	{
		st, body := doReq(t, ts.URL, "PATCH", "/pets/"+petID, delegateID, map[string]any{
			"name": "Milo Updated",
		})
		if st != http.StatusOK {
			t.Fatalf("expected 200 patch pet by delegate, got %d body=%s", st, string(body))
		}
	}

	// 8) Delegado puede crear evento
	eventID := createEvent(t, ts.URL, delegateID, petID, map[string]any{
		"type":        "NOTE",
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"title":       "Check",
		"notes":       "ok",
	})

	// 9) Delegado puede listar eventos
	{
		st, body := doReq(t, ts.URL, "GET", "/pets/"+petID+"/events", delegateID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 list events by delegate, got %d body=%s", st, string(body))
		}
	}

	// 10) Delegado puede void (anular) evento
	{
		st, body := doReq(t, ts.URL, "POST", "/pets/"+petID+"/events/"+eventID+"/void", delegateID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 void event by delegate, got %d body=%s", st, string(body))
		}
	}

	// 11) Owner revoca grant
	{
		st, body := doReq(t, ts.URL, "POST", "/grants/"+grantID+"/revoke", ownerID, nil)
		if st != http.StatusOK {
			t.Fatalf("expected 200 revoke grant by owner, got %d body=%s", st, string(body))
		}
	}

	// 12) Delegado pierde acceso inmediatamente
	{
		st, _ := doReq(t, ts.URL, "GET", "/pets/"+petID, delegateID, nil)
		if st != http.StatusForbidden {
			t.Fatalf("expected 403 get pet after revoke, got %d", st)
		}
	}
	{
		st, _ := doReq(t, ts.URL, "GET", "/pets/"+petID+"/events", delegateID, nil)
		if st != http.StatusForbidden {
			t.Fatalf("expected 403 list events after revoke, got %d", st)
		}
	}
	{
		st, _ := doReq(t, ts.URL, "POST", "/pets/"+petID+"/events", delegateID, map[string]any{
			"type":        "NOTE",
			"occurred_at": time.Now().UTC().Format(time.RFC3339),
			"title":       "Should fail",
		})
		if st != http.StatusForbidden {
			t.Fatalf("expected 403 create event after revoke, got %d", st)
		}
	}
}

func TestHTTP_InviteGrant_RejectsUnknownScope(t *testing.T) {
	ts := httptest.NewServer(router.NewRouter(router.Options{AuthVerifier: nil}))
	defer ts.Close()

	ownerID := "owner-1"
	delegateID := "delegate-1"

	petID := createPet(t, ts.URL, ownerID, map[string]any{
		"name": "Milo",
	})

	// scope inválido => 400
	st, _ := doReq(t, ts.URL, "POST", "/pets/"+petID+"/grants", ownerID, map[string]any{
		"grantee_user_id": delegateID,
		"scopes":          []string{"events:read", "events:unknown"},
	})
	if st != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown scope, got %d", st)
	}
}

func createPet(t *testing.T, baseURL, userID string, payload map[string]any) string {
	t.Helper()

	st, body := doReq(t, baseURL, "POST", "/pets", userID, payload)
	if st != http.StatusCreated {
		t.Fatalf("expected 201 create pet, got %d body=%s", st, string(body))
	}

	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &resp)
	if resp.ID == "" {
		t.Fatalf("create pet: missing id body=%s", string(body))
	}
	return resp.ID
}

func inviteGrant(t *testing.T, baseURL, ownerID, petID, granteeID string, scopes []string) string {
	t.Helper()

	st, body := doReq(t, baseURL, "POST", "/pets/"+petID+"/grants", ownerID, map[string]any{
		"grantee_user_id": granteeID,
		"scopes":          scopes,
	})
	if st != http.StatusCreated {
		t.Fatalf("expected 201 invite grant, got %d body=%s", st, string(body))
	}

	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &resp)
	if resp.ID == "" {
		t.Fatalf("invite grant: missing id body=%s", string(body))
	}
	return resp.ID
}

func createEvent(t *testing.T, baseURL, userID, petID string, payload map[string]any) string {
	t.Helper()

	st, body := doReq(t, baseURL, "POST", "/pets/"+petID+"/events", userID, payload)
	if st != http.StatusCreated {
		t.Fatalf("expected 201 create event, got %d body=%s", st, string(body))
	}

	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &resp)
	if resp.ID == "" {
		t.Fatalf("create event: missing id body=%s", string(body))
	}
	return resp.ID
}

func doReq(t *testing.T, baseURL, method, path, debugUserID string, body any) (int, []byte) {
	t.Helper()

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if debugUserID != "" {
		req.Header.Set("X-Debug-User-ID", debugUserID)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	return res.StatusCode, respBody
}
