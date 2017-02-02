package test

import (
	"testing"
	"time"
)

type sTime struct {
	T time.Time
}

func TestIsDeeplyTime(t *testing.T) {
	now := time.Now()
	got := sTime{now}
	expect := sTime{now.Add(1 * time.Second)}
	same, diff := IsDeeply(got, expect)
	if same {
		t.Error("Times are the same")
	}
	if diff == "" {
		t.Error("Diff is empty")
	}
}
