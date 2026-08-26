package indexbuild

import "testing"

func intPtr(i int) *int { return &i }

func TestBuildBlobsFiltering(t *testing.T) {
	names := GroupNames{
		Groups:    map[string]string{"5": "travel-places", "1": "people-body", "9": "flags"},
		Subgroups: map[string]string{"59": "sky-weather", "28": "person-sport", "98": "country-flag"},
	}
	entries := []RawEmoji{
		{Label: "sun with face", Emoji: "🌞", Tags: []string{"bright", "sun"}, Group: intPtr(5), Subgroup: intPtr(59)},
		{Label: "regional indicator A", Emoji: "🇦", Group: nil, Subgroup: nil},                                            // no group: excluded
		{Label: "light skin tone", Emoji: "🏻", Group: intPtr(2), Subgroup: nil},                                           // component: excluded
		{Label: "man surfing", Emoji: "🏄‍♂️", Group: intPtr(1), Subgroup: intPtr(28), Gender: intPtr(1)},                  // gendered: excluded
		{Label: "person surfing", Emoji: "🏄", Group: intPtr(1), Subgroup: intPtr(28)},                                     // base: kept
		{Label: "flag: United States", Emoji: "🇺🇸", Tags: []string{"US", "flag"}, Group: intPtr(9), Subgroup: intPtr(98)}, // flag: excluded by default
	}

	blobs := BuildBlobs(entries, names, false)
	if len(blobs) != 2 {
		t.Fatalf("got %d blobs, want 2 (sun with face, person surfing): %+v", len(blobs), blobs)
	}

	var sun *Blob
	for i := range blobs {
		if blobs[i].Emoji == "🌞" {
			sun = &blobs[i]
		}
	}
	if sun == nil {
		t.Fatal("sun with face missing from blobs")
	}
	want := "sun with face. bright, sun. travel-places, sky-weather."
	if sun.Text != want {
		t.Errorf("blob text = %q, want %q", sun.Text, want)
	}

	withFlags := BuildBlobs(entries, names, true)
	if len(withFlags) != 3 {
		t.Fatalf("got %d blobs with --include-flags, want 3", len(withFlags))
	}
}
