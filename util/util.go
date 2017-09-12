package util

func SliceDiff(s1, s2 []string) []string {
	var retSlice []string
	m := make(map[string]bool)

	for _, s := range s1 {
		m[s] = true
	}

	for _, s := range s2 {
		if _, ok := m[s]; !ok {
			retSlice = append(retSlice, s)
		}
	}

	return retSlice
}
