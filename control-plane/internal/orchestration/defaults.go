package orchestration

func defaultScheduler(s string) string {
	if s != "" {
		return s
	}
	return "volcano"
}

func defaultPositive(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}

func defaultString(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
