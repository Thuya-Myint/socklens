package platform

import (
	"sync"
	"testing"
)

func TestNotificationStatsBoundedAndConcurrent(t *testing.T) {
	var stats notificationStats
	changes := make(chan struct{}, 1)
	var writers sync.WaitGroup
	for i := 0; i < 1000; i++ {
		writers.Add(1)
		go func() { defer writers.Done(); stats.notify(changes); _ = stats.snapshot() }()
	}
	writers.Wait()
	got := stats.snapshot()
	if len(changes) != 1 || got.Notifications != 1000 || got.Coalesced != 999 {
		t.Fatalf("queue=%d stats=%+v", len(changes), got)
	}
	<-changes
	stats.notify(changes)
	got = stats.snapshot()
	if got.Notifications != 1001 || got.Coalesced != 999 {
		t.Fatalf("stats=%+v", got)
	}
}
