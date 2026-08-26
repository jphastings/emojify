package indexbuild

import (
	"fmt"
	"strconv"
	"strings"
)

// RawEmoji mirrors the subset of emojibase-data's per-entry JSON schema this
// package needs. Skin-tone variants live only in the (deliberately unparsed)
// `skins` array on the base entry — never as top-level entries — so omitting
// that field here is what excludes them.
type RawEmoji struct {
	Label    string   `json:"label"`
	Hexcode  string   `json:"hexcode"`
	Emoji    string   `json:"emoji"`
	Tags     []string `json:"tags"`
	Group    *int     `json:"group"`
	Subgroup *int     `json:"subgroup"`
	Gender   *int     `json:"gender"`
}

// GroupNames maps emojibase's numeric group/subgroup ids to their slugs, from meta/groups.json.
type GroupNames struct {
	Groups    map[string]string `json:"groups"`
	Subgroups map[string]string `json:"subgroups"`
}

// Blob is one candidate emoji's build-time text representation.
type Blob struct {
	Emoji    string
	Label    string
	Group    string
	Subgroup string
	Text     string
}

const componentGroup = 2
const flagsGroup = 9

// BuildBlobs filters entries per the emojify index-build rules (see the
// plan's Global Constraints) and renders each survivor's embeddable text blob.
func BuildBlobs(entries []RawEmoji, names GroupNames, includeFlags bool) []Blob {
	var blobs []Blob
	for _, e := range entries {
		if e.Group == nil {
			continue
		}
		if *e.Group == componentGroup {
			continue
		}
		if e.Gender != nil {
			continue
		}
		if *e.Group == flagsGroup && !includeFlags {
			continue
		}

		group := names.Groups[strconv.Itoa(*e.Group)]
		subgroup := ""
		if e.Subgroup != nil {
			subgroup = names.Subgroups[strconv.Itoa(*e.Subgroup)]
		}

		blobs = append(blobs, Blob{
			Emoji:    e.Emoji,
			Label:    e.Label,
			Group:    group,
			Subgroup: subgroup,
			Text:     fmt.Sprintf("%s. %s. %s, %s.", e.Label, strings.Join(e.Tags, ", "), group, subgroup),
		})
	}
	return blobs
}
