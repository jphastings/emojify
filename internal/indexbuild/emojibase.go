package indexbuild

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const unpkgBase = "https://unpkg.com"

// FetchEmojibaseData downloads en/data.json and meta/groups.json for the
// given emojibase-data version (e.g. "17.0.0") from unpkg.
func FetchEmojibaseData(ctx context.Context, version string) ([]RawEmoji, GroupNames, error) {
	return fetchFrom(ctx, unpkgBase, version)
}

func fetchFrom(ctx context.Context, base, version string) ([]RawEmoji, GroupNames, error) {
	var entries []RawEmoji
	if err := fetchJSON(ctx, fmt.Sprintf("%s/emojibase-data@%s/en/data.json", base, version), &entries); err != nil {
		return nil, GroupNames{}, fmt.Errorf("indexbuild: fetching emoji data: %w", err)
	}

	var names GroupNames
	if err := fetchJSON(ctx, fmt.Sprintf("%s/emojibase-data@%s/meta/groups.json", base, version), &names); err != nil {
		return nil, GroupNames{}, fmt.Errorf("indexbuild: fetching group names: %w", err)
	}

	return entries, names, nil
}

func fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
