package shadow

import (
	"strings"
	"testing"
)

func TestUseAfterFreeError_String(t *testing.T) {
	e := &UseAfterFreeError{
		AllocID:    42,
		AllocSite:  "alloc.go:10",
		FreeSite:   "free.go:20",
		AccessSite: "access.go:30",
		TypeName:   "*Foo",
	}
	s := e.Error()
	for _, want := range []string{"42", "alloc.go:10", "free.go:20", "access.go:30", "*Foo"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestUseAfterFreeError_ArenaVariant(t *testing.T) {
	e := &UseAfterFreeError{
		AllocID:    7,
		AllocSite:  "a.go:1",
		FreeSite:   "a.go:2",
		AccessSite: "a.go:3",
		ArenaID:    3,
		TypeName:   "Bar",
	}
	s := e.Error()
	if !strings.Contains(s, "3") {
		t.Errorf("error string should contain arena ID 3: %q", s)
	}
}

func TestDoubleFreeError_String(t *testing.T) {
	e := &DoubleFreeError{
		AllocID:    99,
		FirstFree:  "free1.go:5",
		SecondFree: "free2.go:6",
		AllocSite:  "alloc.go:1",
	}
	s := e.Error()
	for _, want := range []string{"99", "free1.go:5", "free2.go:6", "alloc.go:1"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestArenaDoubleFreeError_String(t *testing.T) {
	e := &ArenaDoubleFreeError{
		ArenaID:    5,
		FirstFree:  "f1.go:1",
		SecondFree: "f2.go:2",
		CreateSite: "create.go:1",
	}
	s := e.Error()
	for _, want := range []string{"5", "f1.go:1", "f2.go:2", "create.go:1"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestOutOfBoundsError_String(t *testing.T) {
	e := &OutOfBoundsError{
		AllocID:    11,
		AllocSize:  16,
		Offset:     14,
		AccessSize: 4,
		Site:       "oob.go:7",
		TypeName:   "[]byte",
	}
	s := e.Error()
	for _, want := range []string{"11", "16", "14", "oob.go:7", "[]byte"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestUninitializedReadError_String(t *testing.T) {
	e := &UninitializedReadError{
		AllocID:  3,
		Offset:   2,
		Site:     "uninit.go:4",
		TypeName: "int",
	}
	s := e.Error()
	for _, want := range []string{"3", "2", "uninit.go:4", "int"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestUnsafePointerViolation_String(t *testing.T) {
	e := &UnsafePointerViolation{
		Rule:    RuleArithmetic,
		Site:    "unsafe.go:9",
		Details: "pointer out of bounds",
	}
	s := e.Error()
	for _, want := range []string{"unsafe.go:9", "pointer out of bounds"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
	if !strings.Contains(s, "rule 3") {
		t.Errorf("error string should contain rule 3 description: %q", s)
	}
}

func TestEscapedPointerError_String(t *testing.T) {
	e := &EscapedPointerError{
		AllocID:    2,
		ArenaID:    1,
		AllocSite:  "alloc.go:1",
		EscapeSite: "escape.go:5",
		EscapeKind: "return",
	}
	s := e.Error()
	for _, want := range []string{"return", "escape.go:5", "alloc.go:1"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestDataRaceError_String(t *testing.T) {
	e := &DataRaceError{
		AllocID:         4,
		Offset:          0,
		Write1Site:      "w1.go:3",
		Write1Goroutine: 1,
		Write2Site:      "w2.go:4",
		Write2Goroutine: 2,
		TypeName:        "int",
	}
	s := e.Error()
	for _, want := range []string{"w1.go:3", "w2.go:4", "int"} {
		if !strings.Contains(s, want) {
			t.Errorf("error string %q missing %q", s, want)
		}
	}
}

func TestUnsafeRuleStrings(t *testing.T) {
	cases := []struct {
		rule UnsafeRule
		want string
	}{
		{RuleConversion, "rule 1"},
		{RuleUintptr, "rule 2"},
		{RuleArithmetic, "rule 3"},
		{RuleSyscall, "rule 4"},
		{RuleReflect, "rule 5"},
		{RuleSliceHeader, "rule 6"},
	}
	for _, tc := range cases {
		s := tc.rule.String()
		if !strings.Contains(s, tc.want) {
			t.Errorf("rule %d: want %q in %q", tc.rule, tc.want, s)
		}
	}
}
