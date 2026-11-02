package claude

import "strings"

// PublicModel resolves official aliases to the named Claude Code catalog entry.
func PublicModel(id string) (Model, bool) {
	id = DenormalizeModelID(id)
	for _, model := range DefaultModels {
		if model.ID == id {
			return model, true
		}
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
