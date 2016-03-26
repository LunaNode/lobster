package lobster

import "errors"
import "sort"

var regionInterfaces map[string]VmInterface = make(map[string]VmInterface)

func vmGetInterface(region string) VmInterface {
	vmi, ok := regionInterfaces[region]
	if !ok {
		panic(errors.New("no interface registered for " + region))
	}
	return vmi
}

// regions table is used to store transient data about a region
// if a region does not appear in the table, the defaults are assumed:
//  * enabled

type Region struct {
	Region  string
	Enabled bool
}

func regionList() []string {
	var regions []string
	for _, region := range regionListAll() {
		if region.Enabled {
			regions = append(regions, region.Region)
		}
	}
	sort.Sort(sort.StringSlice(regions))
	return regions
}

func regionListAll() []Region {
	var regions []Region
	seenRegions := make(map[string]bool)
	rows := db.Query("SELECT region, enabled FROM regions ORDER BY region")
	for rows.Next() {
		var region Region
		rows.Scan(&region.Region, &region.Enabled)
		regions = append(regions, region)
		seenRegions[region.Region] = true
	}

	for region := range regionInterfaces {
		if !seenRegions[region] {
			regions = append(regions, Region{
				Region:  region,
				Enabled: true,
			})
		}
	}

	return regions
}

func regionEnabled(region string) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM regions WHERE region = ? AND enabled = 0", region).Scan(&count)
	return count == 0
}

func enableRegion(region string) {
	db.Exec("REPLACE INTO regions (region, enabled) VALUES (?, 1)", region)
}

func disableRegion(region string) {
	db.Exec("REPLACE INTO regions (region, enabled) VALUES (?, 0)", region)
}
