// The adapter, against a real http.Server on a real socket, with a real
// httptest.Server standing in for the API.
//
// A test server answers on the loopback, so the socket peer is a bogon and is
// answered locally without a request. Anything that needs a served answer
// therefore has to arrive wearing a public address, through a selector.

package internetdatahttp_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	internetdatahttp "github.com/internetdata/sdk-go-nethttp"
	internetdata "github.com/internetdata/sdk-go/v2"
	"github.com/internetdata/sdk-go/v2/middleware"
)

// The corpus's own address, outside every bogon block, so a lookup of it has to
// reach the stand-in API.
const publicIP = "146.70.116.130"

type answer struct {
	IP       string `json:"ip"`
	Country  string `json:"country"`
	IsVpn    *bool  `json:"is_vpn"`
	IsBogon  bool   `json:"is_bogon"`
	Err      string `json:"err"`
	Attached bool   `json:"attached"`
}

// A stand-in API that answers every lookup with one answer about the address it
// was asked, and records those addresses, which is what the client-address tests
// assert on rather than assume.
type fakeAPI struct {
	mu     sync.Mutex
	asked  []string
	body   map[string]any
	status int
	hang   bool
	server *httptest.Server
}

func newAPI(t *testing.T, body map[string]any) *fakeAPI {
	t.Helper()
	api := &fakeAPI{body: body}
	api.server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	if a.hang {
		<-r.Context().Done()
		return
	}
	ip := strings.TrimPrefix(r.URL.Path, "/")
	a.mu.Lock()
	a.asked = append(a.asked, ip)
	a.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if a.status != 0 {
		w.WriteHeader(a.status)
		fmt.Fprint(w, `{"error":"boom"}`)
		return
	}
	_ = json.NewEncoder(w).Encode(about(a.body, ip))
}

func (a *fakeAPI) addresses() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.asked...)
}

func (a *fakeAPI) client(t *testing.T, extra ...internetdata.Option) *internetdata.Client {
	t.Helper()
	options := append([]internetdata.Option{
		internetdata.WithBaseURL(a.server.URL),
		internetdata.WithoutCache(),
		internetdata.WithRetries(0),
	}, extra...)
	client, err := internetdata.New(options...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// A served answer needs every member the spec requires, so the stand-in serves
// the corpus's own: Unlimited's, the free plan's, and a paid plan's.
func unlimitedAnswer(t *testing.T) map[string]any { return corpusAnswer(t, "unlimited-answer") }

func paidAnswer(t *testing.T) map[string]any { return corpusAnswer(t, "paid-full-shape") }

func corpusAnswer(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/testdata.json")
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	var data struct {
		Lookup []struct {
			Name string         `json:"name"`
			Body map[string]any `json:"body"`
		} `json:"lookup"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parsing the corpus: %v", err)
	}
	for _, c := range data.Lookup {
		if c.Name == name {
			return c.Body
		}
	}
	t.Fatalf("no %s in the corpus's lookup cases", name)
	return nil
}

// body about ip, with members replaced: a copy, so no test changes another's.
func about(body map[string]any, ip string, replaced ...any) map[string]any {
	out := map[string]any{}
	for key, value := range body {
		out[key] = value
	}
	for i := 0; i+1 < len(replaced); i += 2 {
		out[replaced[i].(string)] = replaced[i+1]
	}
	out["ip"] = ip
	return out
}

func fixedIP(*http.Request) string { return publicIP }

// Stands up the middleware in front of a handler that reports what reached it.
func serve(t *testing.T, options internetdatahttp.Options) func(headers map[string]string) (int, answer) {
	t.Helper()
	mw, err := internetdatahttp.New(options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookup := internetdatahttp.FromContext(r.Context())
		out := answer{Attached: lookup != nil}
		if lookup != nil {
			out.IP = lookup.IP
			if lookup.Result != nil {
				out.Country = lookup.Result.Location.Country
				out.IsVpn = lookup.Result.IsVpn
				out.IsBogon = lookup.Result.IsBogon
			}
			if lookup.Err != nil {
				out.Err = lookup.Err.Error()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return func(headers map[string]string) (int, answer) {
		request, err := http.NewRequest(http.MethodGet, server.URL+"/", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		var out answer
		// Checked, not swallowed: when a middleware writes a refusal and then
		// lets the handler run anyway, both bodies land and the result is two
		// concatenated JSON documents. Ignoring the error turns that into a
		// zero-valued answer that looks exactly like a correct refusal.
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("response body is not one JSON document (%v): %s", err, raw)
		}
		return response.StatusCode, out
	}
}

func TestEnrichesTheRequestAndLeavesTheDecisionToTheApp(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: api.client(t), IPSelector: fixedIP,
	}})

	status, out := call(nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !out.Attached || out.Country != "AT" || out.IP != publicIP {
		t.Errorf("answer = %+v, want an attached answer from Austria for %s", out, publicIP)
	}
	if out.IsVpn != nil {
		t.Errorf("is_vpn = %v; Unlimited leaves it out, and absent is not false", *out.IsVpn)
	}
	if got := api.addresses(); len(got) != 1 || got[0] != publicIP {
		t.Errorf("asked about %v, want [%s]", got, publicIP)
	}
}

func TestBlocksOnACountryConditionAndPassesAnotherCountry(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     api.client(t),
		IPSelector: fixedIP,
		BlockCondition: middleware.Conditions{
			{"location": middleware.BlockCondition{"country": internetdatahttp.AnyOf("DE", "AT")}},
		},
	}})
	if status, _ := call(nil); status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}

	allowed := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "DE"}}},
	}})
	if status, _ := allowed(nil); status != http.StatusOK {
		t.Errorf("status = %d, want 200 for another country", status)
	}
}

func TestBlocksOnANetworkTypeAndOnAnonymityAndPassesACleanAnswer(t *testing.T) {
	// The AS's type is a paid plan's: Unlimited's answer carries none, so the
	// block needs the paid body, and on Unlimited the condition can never match.
	byType := middleware.Conditions{{"asn": middleware.BlockCondition{"type": "hosting"}}}
	hosting := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: newAPI(t, paidAnswer(t)).client(t), IPSelector: fixedIP, BlockCondition: byType,
	}})
	if status, _ := hosting(nil); status != http.StatusForbidden {
		t.Errorf("status = %d, want 403 on the network's type", status)
	}
	unlimited := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: newAPI(t, unlimitedAnswer(t)).client(t), IPSelector: fixedIP, BlockCondition: byType,
	}})
	if status, _ := unlimited(nil); status != http.StatusOK {
		t.Errorf("status = %d, want 200: Unlimited carries no AS type, so the condition never matches", status)
	}

	anonymity := middleware.Conditions{{"is_tor": true}, {"is_vpn": true}}
	anonymous := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: newAPI(t, paidAnswer(t)).client(t), IPSelector: fixedIP, BlockCondition: anonymity,
	}})
	if status, _ := anonymous(nil); status != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a VPN", status)
	}

	clean := about(paidAnswer(t), publicIP, "is_vpn", false, "vpn", map[string]any{})
	passing := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: newAPI(t, clean).client(t), IPSelector: fixedIP, BlockCondition: anonymity,
	}})
	if status, _ := passing(nil); status != http.StatusOK {
		t.Errorf("status = %d, want 200 for a clean answer", status)
	}
}

// The handler must not run for a blocked request. Asserting only the status
// would pass even if it did, because the refusal is written first.
func TestABlockedRequestNeverReachesTheHandler(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
	}})
	status, out := call(nil)
	if status != http.StatusForbidden || out.Attached {
		t.Errorf("status = %d, attached = %v; the handler answered a blocked request",
			status, out.Attached)
	}
}

func TestAConditionReachesTheEvidenceFields(t *testing.T) {
	mullvad := newAPI(t, paidAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     mullvad.client(t),
		IPSelector: fixedIP,
		BlockCondition: middleware.Conditions{
			{"vpn": middleware.BlockCondition{"provider": "nordvpn"}},
		},
	}})
	if status, _ := call(nil); status != http.StatusOK {
		t.Errorf("status = %d, want 200; a different provider must not match", status)
	}

	shouted := about(paidAnswer(t), publicIP, "vpn", map[string]any{"provider": "MULLVAD"})
	call2 := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     newAPI(t, shouted).client(t),
		IPSelector: fixedIP,
		BlockCondition: middleware.Conditions{
			{"vpn": middleware.BlockCondition{"provider": "mullvad"}},
		},
	}})
	if status, _ := call2(nil); status != http.StatusForbidden {
		t.Errorf("status = %d, want 403; a provider must compare without case", status)
	}
}

func TestOnBlockedReplacesTheRefusal(t *testing.T) {
	api := newAPI(t, paidAnswer(t))
	call := serve(t, internetdatahttp.Options{
		Options: middleware.Options[*http.Request]{
			Client:         api.client(t),
			IPSelector:     fixedIP,
			BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
		},
		OnBlocked: func(w http.ResponseWriter, _ *http.Request, lookup *middleware.Lookup) {
			w.WriteHeader(http.StatusUnavailableForLegalReasons)
			fmt.Fprintf(w, `{"why":%q}`, lookup.Result.Location.Country)
		},
	})
	status, _ := call(nil)
	if status != http.StatusUnavailableForLegalReasons {
		t.Errorf("status = %d, want 451", status)
	}
}

func TestSkipLeavesTheRequestUntouched(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
		Skip:           func(r *http.Request) bool { return r.URL.Path == "/" },
	}})
	status, out := call(nil)
	if status != http.StatusOK || out.Attached {
		t.Errorf("status = %d, attached = %v; skip should claim the request", status, out.Attached)
	}
	if got := api.addresses(); len(got) != 0 {
		t.Errorf("asked about %v, want nothing", got)
	}
}

func TestAFailingLookupLetsTheVisitorThrough(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	api.status = http.StatusInternalServerError
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
	}})
	status, out := call(nil)
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200; our outage must not become the customer's", status)
	}
	if out.Err == "" {
		t.Error("the reason should be on the context")
	}
}

func TestFailClosedBlocksOnAFailedLookup(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	api.status = http.StatusInternalServerError
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
		FailClosed:     true,
	}})
	if status, _ := call(nil); status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
}

// location is on every plan and its city is not, so on Unlimited a city
// condition names a member the answer does not carry.
func TestOnMissingFieldErrorBecomesA500(t *testing.T) {
	free := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         free.client(t),
		IPSelector:     fixedIP,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"city": "Vienna"}}},
		OnMissingField: middleware.MissingFieldError,
	}})
	if status, _ := call(nil); status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
}

// The test that matters. Every other assertion here would pass whether or not
// the selector is right, because a direct connection has nothing to confuse.
func TestAForgedXForwardedForIsIgnoredByDefault(t *testing.T) {
	forgery := map[string]string{"X-Forwarded-For": publicIP}

	plain := newAPI(t, unlimitedAnswer(t))
	untrusting := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: plain.client(t),
	}})
	_, direct := untrusting(forgery)
	if direct.IP != "127.0.0.1" {
		t.Errorf("resolved %q, want the socket peer; the header is a forgery", direct.IP)
	}
	if got := plain.addresses(); len(got) != 0 {
		t.Errorf("asked about %v; a bogon is answered locally", got)
	}

	explicit := newAPI(t, unlimitedAnswer(t))
	viaSelector := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     explicit.client(t),
		IPSelector: internetdatahttp.XFFIPSelector(0),
	}})
	if _, out := viaSelector(forgery); out.IP != publicIP {
		t.Errorf("resolved %q, want %q", out.IP, publicIP)
	}
}

func TestAHeaderSelectorReadsTheEdgeThatWritesIt(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     api.client(t),
		IPSelector: internetdatahttp.HeaderIPSelector("CF-Connecting-IP"),
	}})
	if _, out := call(map[string]string{"CF-Connecting-IP": "146.70.116.131"}); out.IP != "146.70.116.131" {
		t.Errorf("resolved %q, want 146.70.116.131", out.IP)
	}
	if _, out := call(nil); out.IP != "127.0.0.1" {
		t.Errorf("resolved %q, want the socket peer when the edge wrote no header", out.IP)
	}
}

func TestDepthCountsTrustedHopsFromTheRight(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:     api.client(t),
		IPSelector: internetdatahttp.XFFIPSelector(1),
	}})
	call(map[string]string{
		"X-Forwarded-For": publicIP + ", 70.41.3.18, 150.172.238.178",
	})
	if got := api.addresses(); len(got) != 1 || got[0] != "150.172.238.178" {
		t.Errorf("asked about %v, want [150.172.238.178]", got)
	}
}

// r.RemoteAddr carries a port, and an unstripped one reaches a lookup as a
// malformed address rather than as the client.
func TestTheDefaultSelectorStripsThePort(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client: api.client(t), IPSelector: internetdatahttp.DefaultIPSelector,
	}})
	if _, out := call(nil); out.IP != "127.0.0.1" {
		t.Errorf("resolved %q, want a bare 127.0.0.1", out.IP)
	}
}

func TestAPrivateClientAddressIsAnsweredLocallyAndNeverBlocks(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		BlockCondition: middleware.Conditions{{"is_vpn": true}},
	}})
	status, out := call(nil)
	if status != http.StatusOK {
		t.Errorf("status = %d; local development must not lock you out of your own app", status)
	}
	if !out.IsBogon {
		t.Error("a loopback address should be answered locally")
	}
	if got := api.addresses(); len(got) != 0 {
		t.Errorf("asked about %v, want nothing", got)
	}
}

func TestTheDeadlineBoundsTheRequest(t *testing.T) {
	api := newAPI(t, unlimitedAnswer(t))
	api.hang = true
	call := serve(t, internetdatahttp.Options{Options: middleware.Options[*http.Request]{
		Client:         api.client(t),
		IPSelector:     fixedIP,
		Timeout:        150 * time.Millisecond,
		BlockCondition: middleware.Conditions{{"location": middleware.BlockCondition{"country": "AT"}}},
	}})
	started := time.Now()
	status, out := call(nil)
	if status != http.StatusOK || out.Err == "" {
		t.Errorf("status = %d, err = %q; want a fail-open with the reason recorded", status, out.Err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Errorf("the visitor was held %v, past the budget", elapsed)
	}
}

func TestAConditionThatConstrainsNothingIsRefused(t *testing.T) {
	for _, condition := range []middleware.Conditions{
		{{"is_vpn": false}},
		{{}},
	} {
		if _, err := internetdatahttp.New(internetdatahttp.Options{
			Options: middleware.Options[*http.Request]{BlockCondition: condition},
		}); err == nil {
			t.Errorf("New accepted %v, which would block every request", condition)
		}
	}
}

// The README's app as written: the middleware built from an API key alone, so
// the client, the base URL and the transport are the SDK's defaults, around its
// mux. The default transport answers for the API's host alone, which is why this
// test swaps a global and never runs in parallel. A recorder rather than a
// socket, since only a request built here can come from a public peer.
func TestTheREADMEsAppLooksUpTheVisitorWithOnlyAnAPIKey(t *testing.T) {
	for _, key := range []string{"key", ""} {
		t.Run("key "+map[bool]string{true: "set", false: "unset"}[key != ""], func(t *testing.T) {
			api := &apiHost{body: unlimitedAnswer(t)}
			saved := http.DefaultTransport
			http.DefaultTransport = api
			t.Cleanup(func() { http.DefaultTransport = saved })

			guard := internetdatahttp.Must(internetdatahttp.Options{
				Options: middleware.Options[*http.Request]{APIKey: key},
			})
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				lookup := internetdatahttp.FromContext(r.Context())
				if lookup.Result != nil {
					fmt.Fprintf(w, "Hello, visitor from %s\n", lookup.Result.Location.CountryName)
					return
				}
				fmt.Fprintln(w, "Hello")
			})

			// The socket peer is the visitor, and a forged header names nobody.
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = publicIP + ":4567"
			request.Header.Set("X-Forwarded-For", "146.70.116.131")
			recorder := httptest.NewRecorder()
			guard(mux).ServeHTTP(recorder, request)
			if body := recorder.Body.String(); body != "Hello, visitor from Austria\n" {
				t.Errorf("answered %q, want the visitor's country", body)
			}
			got := api.last()
			if got.path != "/"+publicIP || got.auth != map[bool]string{true: "Bearer key"}[key != ""] {
				t.Errorf("asked %+v, want /%s with the key only when one is set", got, publicIP)
			}
		})
	}
}

// is_bot is a paid plan's, so the README's app blocks on it with a paid key: a
// bot's address is refused before the handler, and another passes, as does any
// address on Unlimited's answer, which carries no is_bot.
func TestTheREADMEsAppBlocksOnIsBot(t *testing.T) {
	for name, want := range map[string]int{"paid-bot": http.StatusForbidden, "unlimited-answer": http.StatusOK} {
		t.Run(name, func(t *testing.T) {
			api := &apiHost{body: corpusAnswer(t, name)}
			saved := http.DefaultTransport
			http.DefaultTransport = api
			t.Cleanup(func() { http.DefaultTransport = saved })

			guard := internetdatahttp.Must(internetdatahttp.Options{
				Options: middleware.Options[*http.Request]{
					APIKey:         "key",
					BlockCondition: middleware.Conditions{{"is_bot": true}},
				},
			})
			reached := false
			handler := guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = publicIP + ":4567"
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != want || reached != (want == http.StatusOK) {
				t.Errorf("status %d with the handler reached %v, want %d", recorder.Code, reached, want)
			}
			if got := api.last(); got.path != "/"+publicIP {
				t.Errorf("asked %+v, want /%s", got, publicIP)
			}
		})
	}
}

// The API's host, answering every lookup with body and recording what it was
// asked. Any other host is refused, so nothing leaves the test.
type apiHost struct {
	mu   sync.Mutex
	body map[string]any
	sent []apiRequest
}

type apiRequest struct{ path, auth string }

func (a *apiHost) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != "api.internetdata.io" {
		return nil, fmt.Errorf("unexpected host %q", request.URL.Host)
	}
	a.mu.Lock()
	a.sent = append(a.sent, apiRequest{path: request.URL.Path, auth: request.Header.Get("Authorization")})
	a.mu.Unlock()
	raw, err := json.Marshal(about(a.body, strings.TrimPrefix(request.URL.Path, "/")))
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(raw))),
		Request:    request,
	}, nil
}

func (a *apiHost) last() apiRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.sent) == 0 {
		return apiRequest{}
	}
	return a.sent[len(a.sent)-1]
}
