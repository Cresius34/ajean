package ajean

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRunCrwlTrustedJSAndSimpleCrawl(t *testing.T) {
	for _, withJS := range []bool{false, true} {
		t.Run(map[bool]string{false: "crawl", true: "execute_js"}[withJS], func(t *testing.T) {
			testHome(t)
			scripts := []string{"(() => { return true; })()"}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer private-crawl-token" {
					t.Error("method/auth")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if withJS {
					if r.URL.Path != "/execute_js" || len(body) != 2 {
						t.Error(r.URL.Path, string(body["crawler_config"]))
					}
					var url string
					var got []string
					json.Unmarshal(body["url"], &url)
					json.Unmarshal(body["scripts"], &got)
					if url != "https://example.org" || !reflect.DeepEqual(got, scripts) {
						t.Error(url, got)
					}
					w.Write([]byte(`{"success":true,"markdown":{"fit_markdown":"trusted result"}}`))
				} else {
					if r.URL.Path != "/crawl" {
						t.Error(r.URL.Path)
					}
					var config struct {
						Params map[string]any `json:"params"`
					}
					json.Unmarshal(body["crawler_config"], &config)
					if _, exists := config.Params["js_code"]; exists {
						t.Error("js_code leaked to /crawl")
					}
					if config.Params["wait_for"] != "css:main" || config.Params["page_timeout"] != float64(4000) {
						t.Error(config.Params)
					}
					w.Write([]byte(`{"results":[{"success":true,"markdown":{"fit_markdown":"simple result"}}]}`))
				}
			}))
			defer server.Close()
			SetConfigKey("CRAWL4AI_URL", server.URL)
			writeCrawlKey("private-crawl-token")
			opts := crwlOptions{waitFor: "css:main", pageTimeoutMs: 4000}
			if withJS {
				opts.jsCode = scripts
			}
			got, err := runCrwl("https://example.org", opts)
			want := map[bool]string{false: "simple result", true: "trusted result"}[withJS]
			if err != nil || got != want {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}
