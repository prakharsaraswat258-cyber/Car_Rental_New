package domain

import "time"

// IST is India Standard Time (UTC+05:30). India has no daylight saving,
// so a fixed zone is exact and needs no tzdata on Windows.
var IST = time.FixedZone("IST", 5*60*60+30*60)
