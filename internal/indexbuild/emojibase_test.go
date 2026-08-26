package indexbuild

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchEmojibaseData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/emojibase-data@1.2.3/en/data.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"label":"sun with face","hexcode":"1F31E","emoji":"🌞","tags":["bright","sun"],"group":5,"subgroup":59}]`))
	})
	mux.HandleFunc("/emojibase-data@1.2.3/meta/groups.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"groups":{"5":"travel-places"},"subgroups":{"59":"sky-weather"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	entries, names, err := fetchFrom(context.Background(), srv.URL, "1.2.3")
	if err != nil {
		t.Fatalf("fetchFrom: %v", err)
	}
	if len(entries) != 1 || entries[0].Label != "sun with face" {
		t.Fatalf("entries = %+v", entries)
	}
	if names.Groups["5"] != "travel-places" {
		t.Fatalf("names = %+v", names)
	}
}
