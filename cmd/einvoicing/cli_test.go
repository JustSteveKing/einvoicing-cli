package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The challenge from internal/altcha's PHP-generated vector, so the login
// test solves a real challenge quickly (counter 37, cost 10).
const challenge = `{"parameters":{"algorithm":"PBKDF2/SHA-256","cost":10,"expiresAt":4102444800,"keyLength":32,"keyPrefix":"74ea8ff470ea31d12435bf3afd6c092c","keySignature":"83bc002378f93868bdbc74e7321a247094f33d427702d1ba9c65fc538f2f44a1","nonce":"00112233445566778899aabbccddeeff","salt":"ffeeddccbbaa99887766554433221100"},"signature":"0532970d1b5d18025b652d4b4137920573149908fa14b7d94c451eb1b1482779"}`

const secret = "einv_live_k3m9x2qa_Zx81QmPq4sVt7LwN2cBy6RfJ0hKd9GuTe3Ao5W"

type fakeAPI struct {
	t        *testing.T
	confirms int
	handlers map[string]http.HandlerFunc
}

func newFakeAPI(t *testing.T) (*httptest.Server, *fakeAPI) {
	f := &fakeAPI{t: t, handlers: map[string]http.HandlerFunc{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := f.handlers[r.Method+" "+r.URL.Path]; ok {
			h(w, r)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server, f
}

func data(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	io.WriteString(w, `{"data":`+body+`}`)
}

func problem(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	io.WriteString(w, body)
}

// runCLI runs the CLI against the fake API with its own config directory.
func runCLI(t *testing.T, server *httptest.Server, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := newApp(strings.NewReader(stdin), &stdout, &stderr)
	code := a.run(append([]string{"--api-url", server.URL}, args...))
	return code, stdout.String(), stderr.String()
}

func useConfigDir(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("EINVOICING_CONFIG_DIR", dir)
	t.Setenv("EINVOICING_API_KEY", "")
	return dir
}

func TestLoginSolvesTheChallengeRetriesAWrongCodeAndSavesTheKey(t *testing.T) {
	dir := useConfigDir(t)
	server, api := newFakeAPI(t)

	api.handlers["POST /v1/sign-in-challenges"] = func(w http.ResponseWriter, r *http.Request) {
		data(w, 200, challenge)
	}
	api.handlers["POST /v1/sign-ins"] = func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Email, Altcha string }
		json.NewDecoder(r.Body).Decode(&body)
		raw, _ := base64.StdEncoding.DecodeString(body.Altcha)
		if body.Email != "steve@example.com" || !strings.Contains(string(raw), `"counter":37`) {
			t.Errorf("sign-in got email %q and payload %s", body.Email, raw)
		}
		data(w, 202, `{"id":"01J9Z3K4Q7VN3XW2R5T6Y8B0CD","email":"steve@example.com","expires_at":"2026-09-11T14:13:11.482Z"}`)
	}
	api.handlers["POST /v1/sign-ins/01J9Z3K4Q7VN3XW2R5T6Y8B0CD/confirmation"] = func(w http.ResponseWriter, r *http.Request) {
		api.confirms++
		if api.confirms == 1 {
			problem(w, 422, `{"type":"https://einvoicing.dev/problems/invalid-code","title":"The code is not correct.","status":422,"attempts_remaining":4}`)
			return
		}
		data(w, 201, `{"account":{"id":"a1","email":"steve@example.com","plan":"free","created_at":"x"},"account_created":true,"key":{"id":"k1","name":"CLI on test","mode":"live","prefix":"einv_live_k3m9x2qa","created_at":"x","last_used_at":null,"expires_at":null,"revoked_at":null,"secret":"`+secret+`"}}`)
	}

	code, stdout, stderr := runCLI(t, server, "000000\n123456\n", "login", "--email", "steve@example.com")
	if code != exitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "4 attempts remain") {
		t.Errorf("expected the wrong code to be reported, got: %s", stderr)
	}
	if !strings.Contains(stdout, "Signed in as steve@example.com (free plan, new account)") {
		t.Errorf("unexpected output: %s", stdout)
	}

	path := filepath.Join(dir, "credentials.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("credentials are %v, want 0600", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), secret) {
		t.Errorf("the key was not saved: %s", raw)
	}
}

func TestValidateExitsOneForAnInvalidDocument(t *testing.T) {
	useConfigDir(t)
	t.Setenv("EINVOICING_API_KEY", secret)
	server, api := newFakeAPI(t)

	api.handlers["POST /v1/validations"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/xml" || r.Header.Get("Authorization") != "Bearer "+secret {
			t.Errorf("unexpected headers %v", r.Header)
		}
		data(w, 200, `{"valid":false,"ruleset":{"id":"peppol-bis-billing-3.0.21","version":"3.0.21"},"document":{"type":"invoice"},"layers":[],"summary":{"errors":1,"warnings":0},"findings":[{"rule_id":"PEPPOL-EN16931-R003","layer":"peppol","severity":"error","message":"A buyer reference or purchase order reference MUST be provided.","explanation":"Peppol needs something the buyer can use to route the invoice.","fix":"Add cbc:BuyerReference.","business_terms":["BT-10"],"location":{"xpath":"/Invoice","line":null,"path":null},"docs_url":"https://einvoicing.dev/rules/PEPPOL-EN16931-R003"}]}`)
	}

	file := filepath.Join(t.TempDir(), "invoice.xml")
	os.WriteFile(file, []byte("<Invoice/>"), 0o644)

	code, stdout, _ := runCLI(t, server, "", "validate", file)
	if code != exitGate {
		t.Fatalf("exit %d, want %d", code, exitGate)
	}
	for _, want := range []string{"INVALID", "PEPPOL-EN16931-R003", "fix: Add cbc:BuyerReference."} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestConvertWrapsABareInvoiceAndListsProblemsWhenItCannotConvert(t *testing.T) {
	useConfigDir(t)
	t.Setenv("EINVOICING_API_KEY", secret)
	server, api := newFakeAPI(t)

	api.handlers["POST /v1/conversions"] = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&body)
		if string(body["target"]) != `"peppol-bis-billing-3"` || body["invoice"] == nil {
			t.Errorf("the bare invoice was not wrapped: %v", body)
		}
		problem(w, 422, `{"type":"https://einvoicing.dev/problems/invalid-invoice","title":"The invoice cannot produce a valid Peppol document.","status":422,"errors":{"invoice.buyer_reference":["A buyer reference is required."]},"findings":[{"rule_id":"PEPPOL-EN16931-R003","layer":"peppol","severity":"error","message":"A buyer reference (BT-10) or purchase order reference (BT-13) MUST be provided.","explanation":"x","fix":null,"business_terms":[],"location":{"xpath":null,"line":null,"path":"invoice.buyer_reference"},"docs_url":"https://einvoicing.dev/rules/PEPPOL-EN16931-R003"}]}`)
	}

	file := filepath.Join(t.TempDir(), "invoice.json")
	os.WriteFile(file, []byte(`{"number":"INV-1"}`), 0o644)

	code, stdout, _ := runCLI(t, server, "", "convert", file)
	if code != exitGate {
		t.Fatalf("exit %d, want %d", code, exitGate)
	}
	if !strings.Contains(stdout, "at invoice.buyer_reference") {
		t.Errorf("expected the JSON path of the problem:\n%s", stdout)
	}
}

func TestProblemsArePrintedAndExitTwo(t *testing.T) {
	useConfigDir(t)
	t.Setenv("EINVOICING_API_KEY", secret)
	server, api := newFakeAPI(t)

	api.handlers["GET /v1/account"] = func(w http.ResponseWriter, r *http.Request) {
		problem(w, 401, `{"type":"https://einvoicing.dev/problems/unauthenticated","title":"No valid API key was presented.","status":401,"detail":"Send an API key."}`)
	}

	code, _, stderr := runCLI(t, server, "", "whoami")
	if code != exitError {
		t.Fatalf("exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "No valid API key was presented.") {
		t.Errorf("problem not printed: %s", stderr)
	}
}

func TestCommandsNeedAKey(t *testing.T) {
	useConfigDir(t)
	server, _ := newFakeAPI(t)

	code, _, stderr := runCLI(t, server, "", "whoami")
	if code != exitError || !strings.Contains(stderr, "einvoicing login") {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
}
