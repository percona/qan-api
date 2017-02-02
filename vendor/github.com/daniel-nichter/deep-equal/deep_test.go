package deep

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	diff, err := Equal("foo", "foo")
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal("foo", "bar")
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestFloat(t *testing.T) {
	diff, err := Equal(1.1, 1.1)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(1.1234561, 1.1234562)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(1.123456, 1.123457)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1.123456 != 1.123457" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestInt(t *testing.T) {
	diff, err := Equal(1, 1)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(1, 2)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1 != 2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestUint(t *testing.T) {
	diff, err := Equal(uint(2), uint(2))
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(uint(2), uint(3))
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "2 != 3" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestBool(t *testing.T) {
	diff, err := Equal(true, true)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(false, false)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(true, false)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "true != false" { // unless you're fipar
		t.Error("wrong diff:", diff[0])
	}
}

func TestTypeMismatch(t *testing.T) {
	type T1 int // same type kind (int)
	type T2 int // but different type
	var t1 T1 = 1
	var t2 T2 = 1
	diff, err := Equal(t1, t2)
	if err == nil {
		t.Error("should return ErrTypeMismatch")
	}
	if err != ErrTypeMismatch {
		t.Error("error != ErrTypeMismatch:", err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep.T1 != deep.T2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestDeepRecursion(t *testing.T) {
	MaxDepth = 2
	defer func() { MaxDepth = 10 }()

	type s3 struct {
		S int
	}
	type s2 struct {
		S s3
	}
	type s1 struct {
		S s2
	}
	foo := map[string]s1{
		"foo": s1{ // 1
			S: s2{ // 2
				S: s3{ // 3
					S: 42, // 4
				},
			},
		},
	}
	bar := map[string]s1{
		"foo": s1{
			S: s2{
				S: s3{
					S: 100,
				},
			},
		},
	}
	diff, err := Equal(foo, bar)
	if err == nil {
		t.Error("should return ErrMaxRecursion")
	}
	if err != ErrMaxRecursion {
		t.Error("error != ErrMaxRecursion:", err)
	}

	MaxDepth = 4
	diff, err = Equal(foo, bar)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo].S.S.S: 42 != 100" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMaxDiff(t *testing.T) {
	MaxDiff = 2
	defer func() { MaxDiff = 5 }()

	a := []int{2, 4, 6, 8, 10}
	b := []int{1, 3, 5, 7, 9}
	diff, err := Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) != MaxDiff {
		t.Error("too many diffs:", len(diff))
	}
}

func TestNotHandled(t *testing.T) {
	a := func(int) {}
	b := func(int) {}
	diff, err := Equal(a, b)
	if err == nil {
		t.Error("should return ErrNotHandled")
	}
	if err != ErrNotHandled {
		t.Error("error != ErrNotHandled:", err)
	}
	if len(diff) > 0 {
		t.Error("got diffs:", diff)
	}
}

func TestStruct(t *testing.T) {
	type s1 struct {
		id     int
		Name   string
		Number int
	}
	sa := s1{
		id:     1,
		Name:   "foo",
		Number: 2,
	}
	sb := sa
	diff, err := Equal(sa, sb)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Name = "bar"
	diff, err = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}

	sb.Number = 22
	diff, err = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}

	sb.id = 11
	diff, err = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}
}

func TestNestedStruct(t *testing.T) {
	type s2 struct {
		Nickname string
	}
	type s1 struct {
		Name  string
		Alias s2
	}
	sa := s1{
		Name:  "Robert",
		Alias: s2{Nickname: "Bob"},
	}
	sb := sa
	diff, err := Equal(sa, sb)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Alias.Nickname = "Bobby"
	diff, err = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Alias.Nickname: Bob != Bobby" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMap(t *testing.T) {
	ma := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	mb := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	diff, err := Equal(ma, mb)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(ma, ma)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	mb["foo"] = 111
	diff, err = Equal(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: 1 != 111" {
		t.Error("wrong diff:", diff[0])
	}

	delete(mb, "foo")
	diff, err = Equal(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: 1 != <does not have key>" {
		t.Error("wrong diff:", diff[0])
	}

	diff, err = Equal(mb, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: <does not have key> != 1" {
		t.Error("wrong diff:", diff[0])
	}

	var mc map[string]int
	diff, err = Equal(ma, mc)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo:1 bar:2] != <nil map>" {
		t.Error("wrong diff:", diff[0])
	}

	diff, err = Equal(mc, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil map> != map[foo:1 bar:2]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestSlice(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}

	diff, err := Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, err = Equal(a, a)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	b[2] = 333
	diff, err = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: 3 != 333" {
		t.Error("wrong diff:", diff[0])
	}

	b = b[0:2]
	diff, err = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: 3 != <no value>" {
		t.Error("wrong diff:", diff[0])
	}

	diff, err = Equal(b, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: <no value> != 3" {
		t.Error("wrong diff:", diff[0])
	}

	var c []int
	diff, err = Equal(a, c)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[1 2 3] != <nil slice>" {
		t.Error("wrong diff:", diff[0])
	}

	diff, err = Equal(c, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil slice> != [1 2 3]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestPointer(t *testing.T) {
	type T struct {
		i int
	}
	a := &T{i: 1}
	b := &T{i: 1}
	diff, err := Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	a = nil
	diff, err = Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil pointer> != deep.T" {
		t.Error("wrong diff:", diff[0])
	}

	a = b
	b = nil
	diff, err = Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep.T != <nil pointer>" {
		t.Error("wrong diff:", diff[0])
	}

	a = nil
	b = nil
	diff, err = Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}
}

func TestTime(t *testing.T) {
	// In an interable kind (i.e. a struct)
	type sTime struct {
		T time.Time
	}
	now := time.Now()
	got := sTime{T: now}
	expect := sTime{T: now.Add(1 * time.Second)}
	diff, err := Equal(got, expect)
	if err != nil {
		t.Error(err)
	}
	if len(diff) != 1 {
		t.Error("expected 1 diff:", diff)
	}

	// Directly
	a := now
	b := now
	diff, err = Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}
}

func TestInterface(t *testing.T) {
	a := map[string]interface{}{
		"foo": map[string]string{
			"bar": "a",
		},
	}
	b := map[string]interface{}{
		"foo": map[string]string{
			"bar": "b",
		},
	}
	diff, err := Equal(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diff) == 0 {
		t.Fatalf("expected 1 diff, got zero")
	}
	if len(diff) != 1 {
		t.Errorf("expected 1 diff, got %d", len(diff))
	}
}
