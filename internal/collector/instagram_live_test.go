package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

// Explicitly opt-in: inspect public profile structure without requesting post
// details. Uses the real shared cooldown and never prints bootstrap tokens.
func TestInstagramLiveProfileStructure(t *testing.T) {
	username := os.Getenv("PMIYC_TEST_INSTAGRAM")
	if username == "" {
		t.Skip("set PMIYC_TEST_INSTAGRAM for a bounded public network diagnostic")
	}
	i := NewInstagram(20*time.Second, nil)
	transport := i.client.Transport
	i.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(req)
		if err != nil {
			return response, err
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
		response.Body.Close()
		response.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			return response, err
		}
		t.Logf("response: %s %s status=%d protocol=%s bytes=%d", req.Method, req.URL.Path, response.StatusCode, response.Proto, len(body))
		if response.StatusCode != 200 {
			return response, nil
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var inspect func(any)
		inspect = func(value any) {
			switch v := value.(type) {
			case map[string]any:
				for key, child := range v {
					if key == "polaris_ordered_timeline_connection" {
						encoded, _ := json.Marshal(child)
						// This timeline is public post metadata, not app tokens.
						if len(encoded) > 12000 {
							encoded = encoded[:12000]
						}
						t.Logf("timeline: %s", encoded)
					}
					if key == "xig_user_by_username" {
						if fields, ok := child.(map[string]any); ok {
							keys := []string{}
							for k := range fields {
								keys = append(keys, k)
							}
							sort.Strings(keys)
							t.Logf("profile fields: %v", keys)
						}
					}
					inspect(child)
				}
			case []any:
				for _, child := range v {
					inspect(child)
				}
			}
		}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "script" && attr(n, "type") == "application/json" {
				var text strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						text.WriteString(c.Data)
					}
				}
				var data any
				decoder := json.NewDecoder(strings.NewReader(text.String()))
				decoder.UseNumber()
				if decoder.Decode(&data) == nil {
					inspect(data)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		return response, nil
	})
	ctx := WithAccess(context.Background(), &AccessOptions{StateDir: "../../data/access-state", MinIntervalMS: 3000, MaxCooldownWaitMS: 30000}, 1)
	r := i.Collect(ctx, Query{Username: username, Budget: 1})
	t.Logf("result: requests=%d status=%s events=%+v", r.Requests, r.Status, r.Events)
	if len(r.Profiles) == 0 {
		t.Fatal("profile not available; see diagnostics")
	}
}
