package lobster

import "fmt"

type Plan struct {
	Id        int
	Name      string
	Price     int64
	Ram       int
	Cpu       int
	Storage   int
	Bandwidth int
	Global    bool

	// region-specific identification from planGet
	Identification string

	// loadable region bindings, if not global
	// maps from region to identification
	RegionPlans map[string]string
}

func (plan *Plan) LoadRegionPlans() {
	if plan.Global {
		return
	}
	rows := db.Query("SELECT region, identification FROM region_plans WHERE plan_id = ?", plan.Id)
	plan.RegionPlans = make(map[string]string)
	for rows.Next() {
		var region, identification string
		rows.Scan(&region, &identification)
		plan.RegionPlans[region] = identification
	}
}

func planListHelper(rows Rows) []*Plan {
	defer rows.Close()
	plans := make([]*Plan, 0)
	for rows.Next() {
		var plan Plan
		rows.Scan(
			&plan.Id,
			&plan.Name,
			&plan.Price,
			&plan.Ram,
			&plan.Cpu,
			&plan.Storage,
			&plan.Bandwidth,
			&plan.Global,
			&plan.Identification,
		)
		plans = append(plans, &plan)
	}
	return plans
}

func planList() []*Plan {
	return planListHelper(
		db.Query(
			"SELECT id, name, price, ram, cpu, storage, bandwidth, global, '' " +
				"FROM plans ORDER BY id",
		),
	)
}

func planListRegion(region string) []*Plan {
	return planListHelper(
		db.Query(
			"SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage,"+
				" plans.bandwidth, plans.global, IFNULL(region_plans.identification, '') "+
				"FROM plans LEFT JOIN region_plans ON plans.id = region_plans.plan_id AND region_plans.region = ? "+
				"WHERE plans.global = 1 OR region_plans.identification IS NOT NULL "+
				"ORDER BY id",
			region,
		),
	)
}

func planGet(planId int) *Plan {
	plans := planListHelper(
		db.Query(
			"SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage,"+
				" plans.bandwidth, plans.global, '' "+
				"FROM plans WHERE id = ?",
			planId,
		),
	)
	if len(plans) == 1 {
		return plans[0]
	} else {
		return nil
	}
}

func planGetRegion(region string, planId int) *Plan {
	plans := planListHelper(
		db.Query(
			"SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage,"+
				" plans.bandwidth, plans.global, IFNULL(region_plans.identification, '') "+
				"FROM plans LEFT JOIN region_plans ON plans.id = region_plans.plan_id AND region_plans.region = ? "+
				"WHERE id = ? AND (plans.global = 1 OR region_plans.identification IS NOT NULL)",
			region, planId,
		),
	)
	if len(plans) == 1 {
		return plans[0]
	} else {
		return nil
	}
}

func planCreate(name string, price int64, ram int, cpu int, storage int, bandwidth int, global bool) int {
	result := db.Exec(
		"INSERT INTO plans (name, price, ram, cpu, storage, bandwidth, global) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, price, ram, cpu, storage, bandwidth, global,
	)
	return result.LastInsertId()
}

func planDelete(planId int) {
	db.Exec("DELETE FROM plans WHERE id = ?", planId)
}

func planAssociateRegion(planId int, region string, identification string) error {
	if _, ok := regionInterfaces[region]; !ok {
		return fmt.Errorf("specified region %s does not exist", region)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM region_plans WHERE plan_id = ? AND region = ?", planId, region).Scan(&count)
	if count == 1 {
		db.Exec("UPDATE region_plans SET identification = ? WHERE plan_id = ? AND region = ?", identification, planId, region)
	} else {
		db.Exec("INSERT INTO region_plans (plan_id, region, identification) VALUES (?, ?, ?)", planId, region, identification)
	}

	return nil
}

func planDeassociateRegion(planId int, region string) {
	db.Exec("DELETE FROM region_plans WHERE plan_id = ? AND region = ?", planId, region)
}

func planAutopopulate(region string) error {
	if _, ok := regionInterfaces[region]; !ok {
		return fmt.Errorf("specified region %s does not exist", region)
	}
	vmi, ok := regionInterfaces[region].(VMIPlans)
	if !ok {
		return L.Error("region_plans_unsupported")
	}
	plans, err := vmi.PlanList()
	if err != nil {
		return err
	}

	// add plans that aren't already having matching identification in database
	for _, plan := range plans {
		var count int
		db.QueryRow(
			"SELECT COUNT(*) FROM region_plans WHERE region = ? AND identification = ?",
			region, plan.Identification,
		).Scan(&count)
		if count == 0 {
			planId := planCreate(plan.Name, plan.Price, plan.Ram, plan.Cpu, plan.Storage, plan.Bandwidth, false)
			planAssociateRegion(planId, region, plan.Identification)
		}
	}

	return nil
}
