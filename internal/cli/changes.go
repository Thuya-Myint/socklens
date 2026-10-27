package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

type socketUpdate struct {
	Before platform.Socket `json:"before"`
	After  platform.Socket `json:"after"`
}

type changeSet struct {
	Timestamp time.Time         `json:"timestamp"`
	Baseline  bool              `json:"baseline"`
	Added     []platform.Socket `json:"added"`
	Removed   []platform.Socket `json:"removed"`
	Updated   []socketUpdate    `json:"updated"`
}

func (c changeSet) hasChanges() bool {
	return len(c.Added)+len(c.Removed)+len(c.Updated) > 0
}

// cloneSockets canonicalizes owner order without modifying the source snapshot.
func cloneSockets(rows []platform.Socket) []platform.Socket {
	result := make([]platform.Socket, len(rows))
	for i, row := range rows {
		row.PIDs = append([]int{}, row.PIDs...)
		sort.Ints(row.PIDs)
		row.Processes = append([]platform.Process{}, row.Processes...)
		sort.Slice(row.Processes, func(i, j int) bool {
			if row.Processes[i].PID != row.Processes[j].PID {
				return row.Processes[i].PID < row.Processes[j].PID
			}
			return row.Processes[i].Name < row.Processes[j].Name
		})
		result[i] = row
	}
	return result
}

type socketRecord struct {
	row       platform.Socket
	signature string
}

func groupSockets(rows []platform.Socket) map[string][]socketRecord {
	groups := make(map[string][]socketRecord)
	for _, row := range cloneSockets(rows) {
		// Endpoint changes or a new inode represent removal/addition. State,
		// PID, and name changes on the same identity represent an update.
		key := row.Protocol + "\x00" + row.Inode + "\x00" + row.Local + "\x00" + row.Remote
		data, _ := json.Marshal(row) // Socket contains only JSON-safe concrete fields.
		groups[key] = append(groups[key], socketRecord{row: row, signature: string(data)})
	}
	for key := range groups {
		group := groups[key]
		sort.Slice(group, func(i, j int) bool { return group[i].signature < group[j].signature })
	}
	return groups
}

// diffSockets preserves multiplicity: identical rows (e.g. inode 0 rows) are
// counted, not overwritten. Exact matches are removed before pairing updates.
func diffSockets(before, after []platform.Socket) changeSet {
	result := changeSet{Added: []platform.Socket{}, Removed: []platform.Socket{}, Updated: []socketUpdate{}}
	oldGroups, newGroups := groupSockets(before), groupSockets(after)
	keySet := map[string]bool{}
	for key := range oldGroups {
		keySet[key] = true
	}
	for key := range newGroups {
		keySet[key] = true
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		old, new := oldGroups[key], newGroups[key]
		var unmatchedOld, unmatchedNew []platform.Socket
		i, j := 0, 0
		for i < len(old) && j < len(new) {
			switch {
			case old[i].signature == new[j].signature:
				i++
				j++
			case old[i].signature < new[j].signature:
				unmatchedOld = append(unmatchedOld, old[i].row)
				i++
			default:
				unmatchedNew = append(unmatchedNew, new[j].row)
				j++
			}
		}
		for ; i < len(old); i++ {
			unmatchedOld = append(unmatchedOld, old[i].row)
		}
		for ; j < len(new); j++ {
			unmatchedNew = append(unmatchedNew, new[j].row)
		}
		paired := len(unmatchedOld)
		if len(unmatchedNew) < paired {
			paired = len(unmatchedNew)
		}
		for n := 0; n < paired; n++ {
			result.Updated = append(result.Updated, socketUpdate{Before: unmatchedOld[n], After: unmatchedNew[n]})
		}
		result.Removed = append(result.Removed, unmatchedOld[paired:]...)
		result.Added = append(result.Added, unmatchedNew[paired:]...)
	}
	return result
}

func writeChanges(out io.Writer, changes changeSet, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(changes)
	}
	if changes.Baseline {
		if err := writeObservationHeading(out, "Baseline", fmt.Sprintf("%s | %s", changes.Timestamp.Format("2006-01-02 15:04:05.000Z07:00"), socketCount(len(changes.Added)))); err != nil {
			return err
		}
		return writeSockets(out, changes.Added, false)
	}
	if err := writeObservationHeading(out, "Changes", fmt.Sprintf("%s | +%d added, -%d removed, %d updated", changes.Timestamp.Format("2006-01-02 15:04:05.000Z07:00"), len(changes.Added), len(changes.Removed), len(changes.Updated))); err != nil {
		return err
	}
	for _, section := range []struct {
		label string
		rows  []platform.Socket
	}{
		{"ADDED", changes.Added}, {"REMOVED", changes.Removed},
	} {
		if len(section.rows) == 0 {
			continue
		}
		if err := writeChangeSection(out, section.label, section.rows); err != nil {
			return err
		}
	}
	if len(changes.Updated) > 0 {
		before, after := make([]platform.Socket, 0, len(changes.Updated)), make([]platform.Socket, 0, len(changes.Updated))
		for _, update := range changes.Updated {
			before = append(before, update.Before)
			after = append(after, update.After)
		}
		if err := writeChangeSection(out, "UPDATED BEFORE", before); err != nil {
			return err
		}
		if err := writeChangeSection(out, "UPDATED AFTER", after); err != nil {
			return err
		}
	}
	return nil
}

func writeChangeSection(out io.Writer, label string, rows []platform.Socket) error {
	role := terminal.Warning
	if label == "ADDED" {
		role = terminal.Success
	} else if label == "REMOVED" {
		role = terminal.Error
	}
	if _, err := fmt.Fprintf(out, "%s (%d)\n", terminal.Style(out, role, label), len(rows)); err != nil {
		return err
	}
	return writeSockets(out, rows, false)
}
