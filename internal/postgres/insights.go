package postgres

import "gorm.io/gorm"

type SpeciesTiming struct {
	Name     string `json:"name"`
	Catches  int    `json:"catches"`
	BestHour int    `json:"best_hour"`
}

func GetSpeciesTimingInsights(db *gorm.DB, days int) ([]SpeciesTiming, error) {
	var results []SpeciesTiming
	sql := `
		SELECT fish_type AS name,
		       SUM(count)::INT AS catches,
		       MODE() WITHIN GROUP (
		         ORDER BY EXTRACT(HOUR FROM caught_at AT TIME ZONE 'Africa/Nairobi')::INT
		       ) AS best_hour
		FROM catch_logs
		WHERE caught_at >= NOW() - (INTERVAL '1 day' * ?)
		  AND fish_type <> '' AND lower(fish_type) <> 'unknown'
		GROUP BY fish_type
		ORDER BY catches DESC
		LIMIT 6`
	err := db.Raw(sql, days).Scan(&results).Error
	return results, err
}
