package requestmodel

import (
	"slices"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// Preserve the old two-pass walk as an independent oracle, including gjson's
// deliberately permissive iteration over incomplete/non-object inputs.
func twoPassCandidates(body []byte, live bool) []string {
	var top, session []string
	collect := func(dst *[]string, k, v gjson.Result) {
		if v.Type == gjson.String && strings.EqualFold(k.String(), "model") {
			if m := strings.TrimSpace(v.String()); m != "" {
				*dst = append(*dst, m)
			}
		}
	}
	gjson.ParseBytes(body).ForEach(func(k, v gjson.Result) bool { collect(&top, k, v); return true })
	gjson.ParseBytes(body).ForEach(func(k, v gjson.Result) bool {
		if strings.EqualFold(k.String(), "session") {
			v.ForEach(func(k, v gjson.Result) bool { collect(&session, k, v); return true })
		}
		return true
	})
	if live && len(session) != 0 || len(top) == 0 {
		return session
	}
	return top
}

func TestFromBodyCandidatesTwoPassParity(t *testing.T) {
	bodies := []string{
		"", "null", " \t[", `[{"model":"ignore"}]`, `"model"`, `{"model":"x"} garbage`,
		`{"model":{},"session":["model","bad"],"MODEL":"\u2003"}`,
		`{"MODEL":"same","sEsSiOn":{"Model":" one ","model":"two"},"m\u006fdel":"same","ſession":{"MODEL":"three"}}`,
		`{"session": null,"model":"usable","session":{"MODEL":"partial","extra":`,
		`{"body":"` + strings.Repeat("0123456789", 1<<15) + `","SESSION":{"MODEL":" large "},"MODEL":"large-top"}`,
	}
	// Cut at every position to cover unfinished keys, escapes and session values.
	sample := `{"x":[{},["]"]],"MoDeL":" early ","SESSION":{"model":"a\u0041","MODEL":"b"},"model":"last"}`
	for i := range len(sample) + 1 {
		bodies = append(bodies, sample[:i])
	}
	for i, body := range bodies {
		for _, route := range []string{"/v1/messages", " /v1/live "} {
			want := twoPassCandidates([]byte(body), IsLiveRequestRoute(route))
			got := FromBodyCandidates(route, "application/json", []byte(body))
			if !slices.Equal(want, got) || (want == nil) != (got == nil) {
				t.Fatalf("body %d route %q: got %#v want %#v", i, route, got, want)
			}
		}
	}
}

func BenchmarkFromBodyCandidatesLarge(b *testing.B) {
	body := []byte(`{"model":"public","input":"` + strings.Repeat("a", 256<<10) + `","session":{"model":" live "}}`)
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		got := FromBodyCandidates("/v1/live", "application/json", body)
		if len(got) != 1 || got[0] != "live" {
			b.Fatal(got)
		}
	}
}
