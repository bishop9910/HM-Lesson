package data

import (
	"strings"
	"testing"
)

func TestWithUTCSession(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			in:   "root:pw@tcp(10.0.0.1:3306)/hm_lesson",
			want: "root:pw@tcp(10.0.0.1:3306)/hm_lesson?time_zone=%27%2B00%3A00%27",
		},
		{
			in:   "root:pw@tcp(10.0.0.1:3306)/hm_lesson?charset=utf8mb4",
			want: "root:pw@tcp(10.0.0.1:3306)/hm_lesson?charset=utf8mb4&time_zone=%27%2B00%3A00%27",
		},
		{
			// 已显式配置过就不重复追加。
			in:   "root:pw@tcp(10.0.0.1:3306)/hm_lesson?time_zone=%27%2B08%3A00%27",
			want: "root:pw@tcp(10.0.0.1:3306)/hm_lesson?time_zone=%27%2B08%3A00%27",
		},
	}
	for _, c := range cases {
		got := withUTCSession(c.in)
		if got != c.want {
			t.Fatalf("withUTCSession(%q) = %q, want %q", c.in, got, c.want)
		}
		if !strings.Contains(got, "time_zone=") {
			t.Fatalf("withUTCSession(%q) 缺少 time_zone", c.in)
		}
	}
}
