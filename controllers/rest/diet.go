package rest

// 식단 일지 API — /api/diet/*. 원본은 노션 「식단 일지」, diet_tb 는 clients/diet.go 가 채우는 복제본.
// 운동일/휴식일 판정: 그날 웨이트 기록(liftexercise_tb) 또는 Apple 운동(workout_tb)이 있으면 운동일,
// 기록이 없어도 오늘이면 운동일(주 6일 루틴 — 저녁 운동 전에 먹는 것도 운동일 목표로), 그 외는 휴식일.
// 추정 소모 = 기초대사량 + Apple active_energy + 섭취의 10%(음식 열효과). 수기 작성 파일.

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"dashboard/clients"
	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type DietController struct {
	controllers.Controller
}

type dietItemView struct {
	clients.Macros
	Id       string  `json:"id"`
	Food     string  `json:"food"`
	Grams    float64 `json:"grams"`
	FoodCode string  `json:"foodCode"`
	Memo     string  `json:"memo"`
	meal     string
	date     string
}

func loadDietItems(conn *models.Connection, where string, args ...interface{}) ([]dietItemView, error) {
	rows, err := conn.Query("SELECT dt_notionid, dt_date, dt_meal, dt_food, dt_grams, dt_kcal, dt_protein, dt_carbs, dt_fat, dt_foodcode, dt_memo "+
		"FROM diet_tb "+where+" ORDER BY dt_date, dt_id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []dietItemView
	for rows.Next() {
		var it dietItemView
		if err := rows.Scan(&it.Id, &it.date, &it.meal, &it.Food, &it.Grams, &it.Kcal, &it.Protein, &it.Carbs, &it.Fat, &it.FoodCode, &it.Memo); err != nil {
			return nil, err
		}
		it.date = it.date[:10]
		list = append(list, it)
	}
	return list, nil
}

// trainingDates 는 from~to 사이 운동 기록이 있는 날짜 → 사유.
func trainingDates(conn *models.Connection, from, to string) map[string]string {
	out := map[string]string{}
	if rows, err := conn.Query("SELECT DISTINCT le_date FROM liftexercise_tb WHERE le_date BETWEEN ? AND ?", from, to); err == nil {
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				out[d[:10]] = "웨이트 기록"
			}
		}
		rows.Close()
	}
	if rows, err := conn.Query("SELECT DISTINCT w_workoutdate FROM workout_tb WHERE w_workoutdate BETWEEN ? AND ?", from, to); err == nil {
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				if _, ok := out[d[:10]]; !ok {
					out[d[:10]] = "Apple 운동"
				}
			}
		}
		rows.Close()
	}
	return out
}

func dietDayType(date, today string, training map[string]string) (string, string) {
	if reason, ok := training[date]; ok {
		return "training", reason
	}
	if date == today {
		return "training", "오늘(운동 예정)"
	}
	return "rest", "운동 기록 없음"
}

func targetFor(t clients.DietTargets, dayType string) clients.Macros {
	if dayType == "training" {
		return t.Training
	}
	return t.Rest
}

// ── GET /api/diet/day ──────────────────────────────────────────────────────

func (c *DietController) Day(date string) {
	today := time.Now().Format("2006-01-02")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		date = today
	}
	conn := c.NewConnection()

	items, err := loadDietItems(conn, "WHERE dt_date = ?", date)
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}
	dayType, reason := dietDayType(date, today, trainingDates(conn, date, date))
	targets := clients.GetDietTargets(conn)

	var totals clients.Macros
	meals := []map[string]interface{}{}
	for _, meal := range clients.DietMeals {
		list := []dietItemView{}
		var sub clients.Macros
		for _, it := range items {
			m := it.meal
			if !containsStr(clients.DietMeals, m) {
				m = "간식" // 끼니 비운 노션 행은 간식으로 보여준다
			}
			if m == meal {
				list = append(list, it)
				sub = sub.Add(it.Macros)
			}
		}
		totals = totals.Add(sub)
		meals = append(meals, map[string]interface{}{"meal": meal, "items": list, "totals": sub.Round()})
	}

	// 자주 먹은 조합 (최근 60일, 음식+양)
	from := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
	recent, _ := loadDietItems(conn, "WHERE dt_date >= ?", from)
	type freq struct {
		it     dietItemView
		count  int
		meals  map[string]int
		lastAt int
	}
	byKey := map[string]*freq{}
	for i, it := range recent {
		key := it.Food + "|" + fmt.Sprintf("%.0f", it.Grams)
		f, ok := byKey[key]
		if !ok {
			f = &freq{meals: map[string]int{}}
			byKey[key] = f
		}
		f.it, f.lastAt = it, i // 값은 가장 최근 기록 기준
		f.count++
		f.meals[it.meal]++
	}
	list := make([]*freq, 0, len(byKey))
	for _, f := range byKey {
		list = append(list, f)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return list[i].lastAt > list[j].lastAt
	})
	if len(list) > 20 {
		list = list[:20]
	}
	frequent := []map[string]interface{}{}
	for _, f := range list {
		bestMeal, best := "", 0
		for m, n := range f.meals {
			if n > best || (n == best && m < bestMeal) {
				bestMeal, best = m, n
			}
		}
		frequent = append(frequent, map[string]interface{}{
			"food": f.it.Food, "grams": f.it.Grams, "foodCode": f.it.FoodCode, "meal": bestMeal, "count": f.count,
			"kcal": f.it.Kcal, "protein": f.it.Protein, "carbs": f.it.Carbs, "fat": f.it.Fat,
		})
	}

	c.Set("date", date)
	c.Set("dayType", dayType)
	c.Set("dayTypeReason", reason)
	c.Set("targets", targetFor(targets, dayType))
	c.Set("totals", totals.Round())
	c.Set("meals", meals)
	c.Set("frequent", frequent)
	c.Set("searchEnabled", clients.FoodSearchEnabled())
}

// ── GET /api/diet/summary?days= ────────────────────────────────────────────

func (c *DietController) Summary(days int) {
	if days <= 0 || days > 180 {
		days = 28
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -(days - 1))
	from := start.Format("2006-01-02")

	conn := c.NewConnection()
	targets := clients.GetDietTargets(conn)
	items, err := loadDietItems(conn, "WHERE dt_date BETWEEN ? AND ?", from, today)
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}
	intake := map[string]clients.Macros{}
	for _, it := range items {
		intake[it.date] = intake[it.date].Add(it.Macros)
	}
	training := trainingDates(conn, from, today)

	metric := map[string]map[string]float64{}
	if rows, err := conn.Query("SELECT hm_metricdate, hm_name, hm_qty FROM healthmetric_tb WHERE hm_name IN ('active_energy','weight') AND hm_metricdate BETWEEN ? AND ?", from, today); err == nil {
		for rows.Next() {
			var d, name string
			var q float64
			if rows.Scan(&d, &name, &q) == nil {
				if metric[d[:10]] == nil {
					metric[d[:10]] = map[string]float64{}
				}
				metric[d[:10]][name] = q
			}
		}
		rows.Close()
	}

	r1 := func(v float64) float64 { return math.Round(v*10) / 10 }
	type dayAgg struct {
		date, dayType string
		logged        bool
		totals        clients.Macros
		target        clients.Macros
		balance       float64
		weight        float64
	}
	var aggs []dayAgg
	dayList := []map[string]interface{}{}
	for t := start; t.Format("2006-01-02") <= today; t = t.AddDate(0, 0, 1) {
		d := t.Format("2006-01-02")
		dt, _ := dietDayType(d, today, training)
		tot, logged := intake[d]
		active := metric[d]["active_energy"]
		weight := r1(metric[d]["weight"])
		var burn, balance interface{}
		a := dayAgg{date: d, dayType: dt, logged: logged, totals: tot.Round(), target: targetFor(targets, dt), weight: weight}
		if logged {
			b := targets.Bmr + active + 0.1*tot.Kcal
			burn, balance = math.Round(b), math.Round(tot.Kcal-b)
			a.balance = tot.Kcal - b
		}
		aggs = append(aggs, a)
		dayList = append(dayList, map[string]interface{}{
			"date": d, "dayType": dt, "logged": logged, "totals": a.totals, "target": a.target,
			"activeEnergy": math.Round(active), "burn": burn, "balance": balance, "weight": weight,
		})
	}

	// 주간 (월요일 시작)
	weekly := []map[string]interface{}{}
	var prevWeight interface{}
	for i := 0; i < len(aggs); {
		t, _ := time.ParseInLocation("2006-01-02", aggs[i].date, time.Local)
		ws := mondayOf(t).Format("2006-01-02")
		var sum clients.Macros
		var balSum, wSum float64
		logged, wCount, proteinHit := 0, 0, 0
		j := i
		for ; j < len(aggs); j++ {
			tj, _ := time.ParseInLocation("2006-01-02", aggs[j].date, time.Local)
			if mondayOf(tj).Format("2006-01-02") != ws {
				break
			}
			a := aggs[j]
			if a.logged {
				logged++
				sum = sum.Add(a.totals)
				balSum += a.balance
				if a.totals.Protein >= a.target.Protein {
					proteinHit++
				}
			}
			if a.weight > 0 {
				wSum += a.weight
				wCount++
			}
		}
		var avg, avgBalance, avgWeight, weightDelta interface{}
		if logged > 0 {
			n := float64(logged)
			avg = clients.Macros{Kcal: sum.Kcal / n, Protein: sum.Protein / n, Carbs: sum.Carbs / n, Fat: sum.Fat / n}.Round()
			avgBalance = math.Round(balSum / n)
		}
		if wCount > 0 {
			w := r1(wSum / float64(wCount))
			avgWeight = w
			if pw, ok := prevWeight.(float64); ok {
				weightDelta = r1(w - pw)
			}
			prevWeight = w
		}
		weekly = append(weekly, map[string]interface{}{
			"weekStart": ws, "loggedDays": logged, "avg": avg, "avgBalance": avgBalance,
			"proteinHitDays": proteinHit, "avgWeight": avgWeight, "weightDelta": weightDelta,
		})
		i = j
	}

	c.Set("targets", map[string]interface{}{"training": targets.Training, "rest": targets.Rest})
	c.Set("bmr", targets.Bmr)
	c.Set("days", dayList)
	c.Set("weekly", weekly)
}

// ── 검색·쓰기·목표 ─────────────────────────────────────────────────────────

func (c *DietController) Search(q string) {
	items, err := clients.SearchFoods(q)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("items", items)
}

func (c *DietController) SaveLog(in clients.DietLogInput) {
	id, err := clients.SaveDietLog(in)
	c.Set("id", id)
	if err != nil {
		log.Error().Str("service", "diet-write").Msg(err.Error())
		c.Error(err)
	}
}

func (c *DietController) DeleteLog(id string) {
	if err := clients.DeleteDietLog(id); err != nil {
		log.Error().Str("service", "diet-write").Msg(err.Error())
		c.Error(err)
	}
}

func (c *DietController) Copy(from, to, meal string) {
	n, err := clients.CopyDiet(from, to, strings.TrimSpace(meal))
	c.Set("count", n)
	if err != nil {
		log.Error().Str("service", "diet-write").Msg(err.Error())
		c.Error(err)
	}
}

func (c *DietController) Targets() {
	t := clients.GetDietTargets(c.NewConnection())
	c.Set("targets", map[string]interface{}{"training": t.Training, "rest": t.Rest})
	c.Set("bmr", t.Bmr)
}

func (c *DietController) SaveTargets(t clients.DietTargets) {
	if err := clients.SaveDietTargets(t); err != nil {
		c.Error(err)
	}
}
