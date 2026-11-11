package claude

import (
	"regexp"
	"strings"
)

var publicModelPattern = regexp.MustCompile(`^claude-([a-z]+)-([0-9]{1,2})(?:-([0-9]{1,2}))?(?:-[0-9]{8})?$`)

// PublicModel resolves official aliases to the named Claude Code catalog entry.
func PublicModel(id string) (Model, bool) {
	id = DenormalizeModelID(id)
	for _, model := range DefaultModels {
		if model.ID == id {
			return model, true
		}
	}
	// Provider-discovered releases must not wait for a gateway release. Keep
	// internal routes, thinking variants and restricted Mythos IDs out of pickers.
	if parts := publicModelPattern.FindStringSubmatch(id); parts != nil && parts[1] != "mythos" {
		version := parts[2]
		if parts[3] != "" {
			version += "." + parts[3]
		}
		family := strings.ToUpper(parts[1][:1]) + parts[1][1:]
		return Model{ID: id, Type: "model", DisplayName: "Claude " + family + " " + version, CreatedAt: "1970-01-01T00:00:00Z"}, true
	}
	return Model{}, false
}

// FilterPublicModelIDs hides internal Claude routes and duplicate snapshots.
// Request routing still accepts their original IDs for existing clients.
func FilterPublicModelIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if strings.Contains(strings.ToLower(id), "claude") {
			model, ok := PublicModel(id)
			if !ok {
				continue
			}
			id = model.ID
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
