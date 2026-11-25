package data

import (
	"encoding/json"
	"testing"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// MySQL 5.7 的 JSON 列不接受空字符串（3140），空列表必须序列化为 []。
func TestEncodeStringListNeverEmpty(t *testing.T) {
	got, err := encodeStringList(nil)
	if err != nil {
		t.Fatalf("encodeStringList(nil) error: %v", err)
	}
	if got != "[]" {
		t.Fatalf("encodeStringList(nil) = %q, want \"[]\"", got)
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("encodeStringList(nil) is not valid JSON: %q", got)
	}

	got, err = encodeStringList([]string{"a", "b"})
	if err != nil {
		t.Fatalf("encodeStringList([a b]) error: %v", err)
	}
	if got != `["a","b"]` {
		t.Fatalf("encodeStringList([a b]) = %q", got)
	}
}

func TestDecodeStringList(t *testing.T) {
	if got := decodeStringList(nil); got != nil {
		t.Fatalf("decodeStringList(nil) = %v, want nil", got)
	}
	if got := decodeStringList([]byte("[]")); len(got) != 0 {
		t.Fatalf("decodeStringList([]) = %v, want empty", got)
	}
	got := decodeStringList([]byte(`["x","y"]`))
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("decodeStringList([x y]) = %v", got)
	}
	if got := decodeStringList([]byte("not-json")); got != nil {
		t.Fatalf("decodeStringList(bad) = %v, want nil", got)
	}
}

func TestEncodeDecodeAudience(t *testing.T) {
	encoded, err := encodeAudience("", "")
	if err != nil {
		t.Fatalf("encodeAudience error: %v", err)
	}
	// 未指定受众时默认 all，且必须是合法 JSON。
	if encoded != `{"type":"all","value":""}` {
		t.Fatalf("encodeAudience default = %q", encoded)
	}
	if !json.Valid([]byte(encoded)) {
		t.Fatalf("encodeAudience is not valid JSON: %q", encoded)
	}
	if got := decodeAudienceValue("all", []byte(encoded)); got != "" {
		t.Fatalf("decodeAudienceValue default = %q, want empty", got)
	}

	encoded, err = encodeAudience(biz.AudienceGroup, biz.GroupClassActive)
	if err != nil {
		t.Fatalf("encodeAudience group error: %v", err)
	}
	if got := decodeAudienceValue(biz.AudienceGroup, []byte(encoded)); got != biz.GroupClassActive {
		t.Fatalf("decodeAudienceValue group = %q", got)
	}
	if got := decodeAudienceValue(biz.AudienceGroup, []byte("bad")); got != "" {
		t.Fatalf("decodeAudienceValue bad = %q, want empty", got)
	}
}

// 没有出勤记录时要写 {}，否则 JSON 列同样会报 3140。
func TestEncodeAttendanceNeverEmpty(t *testing.T) {
	got, err := encodeAttendance(nil)
	if err != nil {
		t.Fatalf("encodeAttendance(nil) error: %v", err)
	}
	if got != "{}" {
		t.Fatalf("encodeAttendance(nil) = %q, want {}", got)
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("encodeAttendance(nil) is not valid JSON: %q", got)
	}
	if rows := decodeAttendance([]byte(got), nil); len(rows) != 0 {
		t.Fatalf("decodeAttendance({}) = %v, want empty", rows)
	}

	rows, err := encodeAttendance([]biz.AttendanceRow{
		{StudentID: "s1", Status: biz.AttendancePresent},
		{StudentID: "s2", Status: biz.AttendanceAbsent},
	})
	if err != nil {
		t.Fatalf("encodeAttendance rows error: %v", err)
	}
	decoded := decodeAttendance([]byte(rows), map[string]string{"s1": "甲", "s2": "乙"})
	if len(decoded) != 2 {
		t.Fatalf("decodeAttendance = %v, want 2 rows", decoded)
	}
	// 解码按学员 id 升序，保证出勤列表顺序稳定。
	if decoded[0].StudentID != "s1" || decoded[0].StudentName != "甲" || decoded[0].Status != biz.AttendancePresent {
		t.Fatalf("decoded[0] = %+v", decoded[0])
	}
	if decoded[1].StudentID != "s2" || decoded[1].Status != biz.AttendanceAbsent {
		t.Fatalf("decoded[1] = %+v", decoded[1])
	}
}
