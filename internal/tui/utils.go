package tui

// StringsRepeat repeats a string n times.
func StringsRepeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
