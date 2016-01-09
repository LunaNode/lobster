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

	db *Database
}

func (plan *Plan) LoadRegionPlans() {
	if plan.Global {
		return
	}
	rows := plan.db.Query("SELECT region, identification FROM region_plans WHERE plan_id = ?", plan.Id)
	plan.RegionPlans = make(map[string]string)
	for rows.Next() {
		var region, identification string
		rows.Scan(&region, &identification)
		plan.RegionPlans[region] = identification
	}
}

func planListHelper(db *Database, rows Rows) []*Plan {
	defer rows.Close()
	plans := make([]*Plan, 0)
	for rows.Next() {
		plan := Plan{db: db}
		rows.Scan(&plan.Id, &plan.Name, &plan.Price, &plan.Ram, &plan.Cpu, &plan.Storage, &plan.Bandwidth, &plan.Global, &plan.Identification)
		plans = append(plans, &plan)
	}
	return plans
}

func planList(db *Database) []*Plan {
	return planListHelper(db, db.Query("SELECT id, name, price, ram, cpu, storage, bandwidth, global, '' FROM plans ORDER BY id"))
}

func planListRegion(db *Database, region string) []*Plan {
	return planListHelper(db, db.Query("SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage, plans.bandwidth, plans.global, IFNULL(region_plans.identification, '') FROM plans LEFT JOIN region_plans ON plans.id = region_plans.plan_id AND region_plans.region = ? WHERE plans.global = 1 OR region_plans.identification IS NOT NULL ORDER BY id", region))
}

func planGet(db *Database, planId int) *Plan {
	plans := planListHelper(db, db.Query("SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage, plans.bandwidth, plans.global, '' FROM plans WHERE id = ?", planId))
	if len(plans) == 1 {
		return plans[0]
	} else {
		return nil
	}
}

func planGetRegion(db *Database, region string, planId int) *Plan {
	plans := planListHelper(db, db.Query("SELECT plans.id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage, plans.bandwidth, plans.global, IFNULL(region_plans.identification, '') FROM plans LEFT JOIN region_plans ON plans.id = region_plans.plan_id AND region_plans.region = ? WHERE id = ? AND (plans.global = 1 OR region_plans.identification IS NOT NULL)", region, planId))
	if len(plans) == 1 {
		return plans[0]
	} else {
		return nil
	}
}

func planCreate(db *Database, name string, price int64, ram int, cpu int, storage int, bandwidth int, global bool) int {
	result := db.Exec("INSERT INTO plans (name, price, ram, cpu, storage, bandwidth, global) VALUES (?, ?, ?, ?, ?, ?, ?)", name, price, ram, cpu, storage, bandwidth, global)
	return result.LastInsertId()
}

func planDelete(db *Database, planId int) {
	db.Exec("DELETE FROM plans WHERE id = ?", planId)
}

func planAssociateRegion(db *Database, planId int, region string, identification string) error {
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

func planDeassociateRegion(db *Database, planId int, region string) {
	db.Exec("DELETE FROM region_plans WHERE plan_id = ? AND region = ?", planId, region)
}

func planAutopopulate(db *Database, region string) error {
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
		db.QueryRow("SELECT COUNT(*) FROM region_plans WHERE region = ? AND identification = ?", region, plan.Identification).Scan(&count)
		if count == 0 {
			planId := planCreate(db, plan.Name, plan.Price, plan.Ram, plan.Cpu, plan.Storage, plan.Bandwidth, false)
			planAssociateRegion(db, planId, region, plan.Identification)
		}
	}

	return nil
}
