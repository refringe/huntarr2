package arr

// qualityRank builds a map from quality/group ID to ordinal rank within
// a profile. Higher rank means higher quality. Qualities within a group
// share the same rank, and the group's own ID also maps to that rank so
// that a profile's Cutoff field can reference either a quality or a
// group. Only allowed entries are included.
func qualityRank(profile QualityProfile) map[int]int {
	ranks := make(map[int]int)
	for i, entry := range profile.Items {
		if !entry.Allowed {
			continue
		}
		if entry.Quality != nil {
			ranks[entry.Quality.ID] = i
			continue
		}
		// Group entry: map the group ID and each child quality ID to
		// the same rank.
		if entry.ID != 0 {
			ranks[entry.ID] = i
		}
		for _, child := range entry.Items {
			if child.Quality != nil {
				ranks[child.Quality.ID] = i
			}
		}
	}
	return ranks
}

// cutoffRank returns the ordinal rank of the profile's Cutoff quality
// (or group) within the given rank map. Returns -1 if the cutoff ID is
// not present in the allowed entries, which causes all items in that
// profile to be treated as not upgradeable.
func cutoffRank(profile QualityProfile, ranks map[int]int) int {
	if rank, ok := ranks[profile.Cutoff]; ok {
		return rank
	}
	return -1
}

// filterUpgradeable returns items whose current quality rank is below
// the profile's cutoff. It skips items without files, unmonitored
// items, items with no known quality IDs, and items whose profile has
// UpgradeAllowed set to false. For items with multiple quality IDs
// (Lidarr albums with multiple track files) the lowest ranked quality
// is used as the representative. The returned FilterStats report how
// many items were excluded at each stage.
func filterUpgradeable(
	items []LibraryItem,
	profiles map[int]QualityProfile,
) ([]UpgradeItem, FilterStats) {
	type profileCache struct {
		ranks          map[int]int
		cutoff         int
		upgradeAllowed bool
	}

	cache := make(map[int]*profileCache, len(profiles))
	for id, p := range profiles {
		r := qualityRank(p)
		cache[id] = &profileCache{
			ranks:          r,
			cutoff:         cutoffRank(p, r),
			upgradeAllowed: p.UpgradeAllowed,
		}
	}

	var stats FilterStats
	stats.LibraryTotal = len(items)

	var result []UpgradeItem
	for _, item := range items {
		if !item.HasFile {
			stats.NoFile++
			continue
		}
		if !item.Monitored {
			stats.Unmonitored++
			continue
		}

		pc, ok := cache[item.QualityProfileID]
		if !ok {
			stats.NoProfile++
			continue
		}
		if !pc.upgradeAllowed {
			stats.UpgradeBlocked++
			continue
		}

		// Find the minimum rank across all quality IDs for this item.
		// For movies and episodes there is exactly one; for Lidarr
		// albums there may be several.
		lowest := -1
		for _, qid := range item.CurrentQualityIDs {
			rank, known := pc.ranks[qid]
			if !known {
				continue
			}
			if lowest < 0 || rank < lowest {
				lowest = rank
			}
		}

		if lowest < 0 {
			stats.UnknownQuality++
			continue
		}

		if lowest < pc.cutoff {
			result = append(result, UpgradeItem{
				ID:         item.ID,
				Label:      item.Label,
				DetailPath: item.DetailPath,
			})
		} else {
			stats.AtOrAbove++
		}
	}

	stats.Upgradeable = len(result)
	return result, stats
}
