package platform

// fillMissingProcessNames preserves preferred observations and fills only empty
// names. It does not infer executable paths or process identity from a basename.
func fillMissingProcessNames(rows []Socket, names map[int]string) {
	for i := range rows {
		for j := range rows[i].Processes {
			process := &rows[i].Processes[j]
			if process.Name == "" {
				process.Name = names[process.PID]
			}
		}
	}
}
