package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPythonCaptionGolden(t *testing.T) {
	b, err := os.ReadFile("testdata/caption-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Fixtures []struct {
			Input    *string  `json:"input"`
			Caption  *string  `json:"caption"`
			Mentions []string `json:"mentions"`
			Hashtags []string `json:"hashtags"`
		}
	}
	if err = json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	for i, f := range golden.Fixtures {
		var caption any
		if f.Input != nil {
			caption = map[string]any{"text": *f.Input}
		}
		p, err := parsePost(postJSON("TEST", caption), "TEST")
		if err != nil || !reflect.DeepEqual(p.Caption, f.Caption) || !reflect.DeepEqual(p.Mentions, f.Mentions) || !reflect.DeepEqual(p.Hashtags, f.Hashtags) {
			t.Errorf("fixture %d: got %+v error=%v want %+v", i, p, err, f)
		}
	}
}

func profileHTML(private bool, timeline string) string {
	return fmt.Sprintf(`<html><script data-test="true" type='application/json'>{"require":[{"__bbox":{"result":{"data":{"xig_user_by_username":{"pk":"12345678901234567","id":"opaque-page-id","username":"testuser","full_name":"Test User","biography":"","is_private":%t,"all_media_count":null%s}}}}}]}</script></html>`, private, timeline)
}
func timeline(codes []string, next bool, cursor string) string {
	edges := []map[string]any{}
	for _, code := range codes {
		edges = append(edges, map[string]any{"node": map[string]any{"code": code}})
	}
	b, _ := json.Marshal(map[string]any{"edges": edges, "page_info": map[string]any{"has_next_page": next, "end_cursor": cursor}})
	return string(b)
}
func postJSON(code string, caption any) map[string]any {
	return map[string]any{"xdt_api__v1__media__shortcode__web_info": map[string]any{"items": []any{map[string]any{"code": code, "taken_at": json.Number("1740322502"), "caption": caption, "user": map[string]any{"pk": "12345678901234567", "username": "testuser"}}}}}
}
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c := newClient()
	c.base = s.URL
	c.interval = 0
	return c
}
func TestPrivateProfileNeverRequestsPosts(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, profileHTML(true, "")) })
	r := c.Collect(context.Background(), "testuser", 3)
	if calls != 1 || r.ProfileStatus != "SUCCESS" || r.PostsStatus != "PRIVATE_PROFILE_NOT_REQUESTED" || r.Profile.PostCount != nil {
		t.Fatalf("private/null handling: %+v", r)
	}
}
func TestBlockedResponsesStopWithoutRetry(t *testing.T) {
	for _, status := range []int{302, 401, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/accounts/login/")
				w.Header().Set("Retry-After", "900")
				w.WriteHeader(status)
			})
			r := c.Collect(context.Background(), "testuser", 3)
			if calls != 1 || r.Error == nil || r.PostsStatus != "NOT_ATTEMPTED" || r.Requests[0].Status != status {
				t.Fatalf("retried/ignored failure: %+v", r)
			}
		})
	}
}
func TestMissingOrWrongProfileIsNotEmpty(t *testing.T) {
	for _, html := range []string{`<html>Log in</html>`, strings.ReplaceAll(profileHTML(false, ""), "testuser", "otherperson")} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
		r := c.Collect(context.Background(), "testuser", 3)
		if r.ProfileStatus != "FAILED" || r.Error == nil {
			t.Fatalf("bad identity accepted: %+v", r)
		}
	}
}
func TestCaptionIntegrity(t *testing.T) {
	p, err := parsePost(postJSON("ABC", map[string]any{"text": "Cafe\u0301 🥂 @User.Name #JAKARTA #日本語"}), "ABC")
	if err != nil || p.Caption == nil || *p.Caption != "Café 🥂 @User.Name #JAKARTA #日本語" || !reflect.DeepEqual(p.Mentions, []string{"user.name"}) || !reflect.DeepEqual(p.Hashtags, []string{"jakarta", "日本語"}) {
		t.Fatalf("unicode/mentions damaged: %+v %v", p, err)
	}
	p, err = parsePost(postJSON("ABC", nil), "ABC")
	if err != nil || p.Caption != nil || len(p.Mentions) != 0 {
		t.Fatalf("null changed: %+v %v", p, err)
	}
	data := postJSON("ABC", nil)
	delete(object(array(object(data["xdt_api__v1__media__shortcode__web_info"])["items"])[0]), "caption")
	if _, err = parsePost(data, "ABC"); err == nil {
		t.Fatal("missing caption treated as null")
	}
	if _, err = parsePost(postJSON("OTHER", nil), "ABC"); err == nil {
		t.Fatal("wrong post accepted")
	}
}

func TestPublisherMismatchDoesNotBindPost(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testuser/":
			fmt.Fprint(w, profileHTML(false, `,"polaris_ordered_timeline_connection":`+timeline([]string{"ONE"}, false, "")))
		case "/":
			fmt.Fprint(w, `"LSD",[],{"token":"fixture"};"IG_USER_EIMU":"0"`)
		default:
			data := postJSON("ONE", nil)
			node := object(array(object(data["xdt_api__v1__media__shortcode__web_info"])["items"])[0])
			node["user"] = map[string]any{"pk": "someone-else", "username": "testuser"}
			json.NewEncoder(w).Encode(map[string]any{"data": data})
		}
	})
	r := c.Collect(context.Background(), "testuser", 3)
	if r.ProfileStatus != "SUCCESS" || r.PostsStatus != "FAILED" || len(r.Posts) != 0 || r.Error == nil || !strings.Contains(r.Error.Message, "publisher") {
		t.Fatalf("wrong publisher accepted: %+v", r)
	}
}
func TestMentionBoundaries(t *testing.T) {
	m, h := captionTags("mail@example.com @A..B @user. _@Other @x #Tag #TAG")
	if !reflect.DeepEqual(m, []string{"a", "user", "other", "x"}) || !reflect.DeepEqual(h, []string{"tag", "tag"}) {
		t.Fatalf("%v %v", m, h)
	}
}
func TestPageQueryOrderAndMalformedScripts(t *testing.T) {
	html := `<script type="application/json">{bad}</script><script type="application/json">{"a":{"__bbox":{"result":{"data":{"xig_user_by_username":{"pk":"1","username":"testuser","full_name":"first","is_private":false}}}}},"b":{"__bbox":{"result":{"data":{"xig_user_by_username":{"full_name":"second"}}}}}}</script>`
	for i := 0; i < 20; i++ {
		p, _, _, err := parseProfile([]byte(html), "testuser")
		if err != nil || p.FullName != "second" {
			t.Fatalf("order not stable: %+v %v", p, err)
		}
	}
}
func TestPaginationAndSampleLimit(t *testing.T) {
	for _, limit := range []int{1, 3} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			postCalls, pageCalls := 0, 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/testuser/":
					fmt.Fprint(w, profileHTML(false, `,"polaris_ordered_timeline_connection":`+timeline([]string{"ONE"}, true, "cursor1")))
				case "/":
					http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "fixturecsrf", Path: "/"})
					fmt.Fprint(w, `<script>"LSD",[],{"token":"fixturelsd"};"IG_USER_EIMU":"0"</script>`)
				case "/api/graphql":
					if r.Method != "POST" || r.Header.Get("X-FB-LSD") != "fixturelsd" || r.Header.Get("X-CSRFToken") != "fixturecsrf" {
						t.Error("incorrect request headers")
					}
					r.ParseForm()
					var vars map[string]any
					json.Unmarshal([]byte(r.Form.Get("variables")), &vars)
					if r.Form.Get("doc_id") == "28904005159232689" {
						pageCalls++
						if vars["after"] != "cursor1" || vars["id"] != "opaque-page-id" {
							t.Error("wrong pagination binding")
						}
						fmt.Fprint(w, `{"data":{"node":{"polaris_ordered_timeline_connection":`+timeline([]string{"ONE", "TWO", "THREE", "FOUR"}, false, "")+`}}}`)
					} else {
						postCalls++
						json.NewEncoder(w).Encode(map[string]any{"data": postJSON(stringValue(vars["shortcode"]), nil)})
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(500)
				}
			})
			r := c.Collect(context.Background(), "testuser", limit)
			if r.Error != nil || len(r.Posts) != limit || postCalls != limit || r.StopReason != "sample_limit" {
				t.Fatalf("bad limit: %+v", r)
			}
			if limit == 1 && pageCalls != 0 || limit == 3 && pageCalls != 1 {
				t.Fatalf("unexpected page requests %d", pageCalls)
			}
		})
	}
}
func TestMissingTimelineNotEmpty(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, profileHTML(false, "")) })
	r := c.Collect(context.Background(), "testuser", 3)
	if r.ProfileStatus != "SUCCESS" || r.PostsStatus != "FAILED" {
		t.Fatalf("%+v", r)
	}
}

func embeddedTimeline(code, owner string, caption any, next bool) string {
	node := map[string]any{"code": code, "user": map[string]any{"id": owner, "username": "testuser"}, "caption": caption}
	encoded, _ := json.Marshal(map[string]any{"edges": []any{map[string]any{"node": node}}, "page_info": map[string]any{"has_next_page": next, "end_cursor": "cursor1"}})
	return string(encoded)
}

func TestEmbeddedCaptionNeedsOnlyProfileRequest(t *testing.T) {
	for _, hasCaption := range []bool{true, false} {
		t.Run(fmt.Sprint(hasCaption), func(t *testing.T) {
			var caption any
			if hasCaption {
				caption = map[string]any{"text": "Cafe\u0301 @Teman  kemarin\n#Acara"}
			}
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/testuser/" {
					t.Fatalf("unnecessary API request: %s", r.URL.Path)
				}
				fmt.Fprint(w, profileHTML(false, `,"polaris_ordered_timeline_connection":`+embeddedTimeline("ONE", "opaque-page-id", caption, false)))
			})
			r := c.Collect(context.Background(), "testuser", 3)
			if r.Error != nil || calls != 1 || r.PostsStatus != "SUCCESS" || r.StopReason != "source_end" || len(r.Posts) != 1 {
				t.Fatalf("%+v calls=%d", r, calls)
			}
			p := r.Posts[0]
			if p.SourceKind != "timeline" || p.PublisherIDType != "web_id" || p.PublisherID != r.Profile.WebID || p.PublishedAt != "" || p.TimestampStatus != "UNAVAILABLE" || p.Receipt.SHA256 != r.Requests[0].SHA256 || p.Receipt.URL != r.Requests[0].URL {
				t.Fatalf("invented time/identity/provenance: %+v", p)
			}
			if hasCaption && (p.Caption == nil || *p.Caption != "Café @Teman  kemarin\n#Acara") || !hasCaption && p.Caption != nil {
				t.Fatalf("caption changed: %+v", p)
			}
		})
	}
}

func TestEmbeddedPublisherMismatchStopsWithoutDetailAPI(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, profileHTML(false, `,"polaris_ordered_timeline_connection":`+embeddedTimeline("ONE", "another-account", nil, false)))
	})
	r := c.Collect(context.Background(), "testuser", 3)
	if calls != 1 || len(r.Posts) != 0 || r.Error == nil || !strings.Contains(r.Error.Message, "publisher") {
		t.Fatalf("mismatch hidden or retried: %+v", r)
	}
}

func TestEmbeddedPaginationKeepsSeparateResponseProof(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testuser/":
			fmt.Fprint(w, profileHTML(false, `,"polaris_ordered_timeline_connection":`+embeddedTimeline("ONE", "opaque-page-id", nil, true)))
		case "/":
			fmt.Fprint(w, `"LSD",[],{"token":"fixture"};"IG_USER_EIMU":"0"`)
		case "/api/graphql":
			r.ParseForm()
			if r.Form.Get("doc_id") != "28904005159232689" {
				t.Fatal("requested redundant detail API")
			}
			fmt.Fprint(w, `{"data":{"node":{"polaris_ordered_timeline_connection":`+embeddedTimeline("TWO", "opaque-page-id", map[string]any{"text": "Second"}, false)+`}}}`)
		default:
			t.Fatal("unexpected request")
		}
	})
	r := c.Collect(context.Background(), "testuser", 2)
	if r.Error != nil || len(r.Requests) != 3 || len(r.Posts) != 2 {
		t.Fatalf("%+v", r)
	}
	if r.Posts[0].Receipt.URL != c.base+"/testuser/" || r.Posts[1].Receipt.URL != c.base+"/api/graphql" || r.Posts[1].Receipt.SHA256 != r.Requests[2].SHA256 {
		t.Fatalf("pagination provenance mixed: %+v", r.Posts)
	}
}
func TestCancellationAndResponseLimit(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", maxBody+1)) })
	r := c.Collect(context.Background(), "testuser", 3)
	if r.Error == nil || !strings.Contains(r.Error.Message, "8 MiB") {
		t.Fatalf("body not bounded: %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = c.Collect(ctx, "testuser", 3)
	if r.Error == nil {
		t.Fatal("ignored cancellation")
	}
}
