package arr

// qualityRank builds a map from quality/group ID to ordinal rank within a profile; higher rank means higher
// quality. Qualities within a group share the same rank, the group's own ID also maps to that rank, and only
// allowed entries are included.
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

// cutoffRank returns the rank of the profile's Cutoff entry, or -1 when it is not among the allowed entries.
func cutoffRank(profile QualityProfile, ranks map[int]int) int {
	if rank, ok := ranks[profile.Cutoff]; ok {
		return rank
	}
	return -1
}

// filterUpgradeable returns items whose current quality rank is below the profile's cutoff, skipping items without
// files, unmonitored items, items with no known quality IDs, and items whose profile blocks upgrades. Items with
// multiple quality IDs (Lidarr albums) are represented by their lowest ranked quality. The returned FilterStats
// report how many items were excluded at each stage.
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

// filterMissing returns monitored items that have no file at all.
func filterMissing(items []LibraryItem) []UpgradeItem {
	var result []UpgradeItem
	for _, item := range items {
		if item.HasFile || !item.Monitored {
			continue
		}
		result = append(result, UpgradeItem{
			ID:         item.ID,
			Label:      item.Label,
			DetailPath: item.DetailPath,
		})
	}
	return result
}
