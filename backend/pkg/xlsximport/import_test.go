package xlsximport

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestTemplateHasHeaders(t *testing.T) {
	data, err := Template()
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	got, err := f.GetCellValue(sheet, "A1")
	if err != nil {
		t.Fatalf("read A1: %v", err)
	}
	if got != "学员姓名" {
		t.Fatalf("A1 = %q, want 学员姓名", got)
	}
}

func TestParseMapsRow(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	headers := []string{"学员姓名", "年级", "学校", "家长姓名", "家长手机号", "家长飞书账号", "来源渠道", "客户分组", "自定义标签", "当前状态", "下次回访时间", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	values := []string{"陈思远", "高二", "南通中学", "陈丽", "13800000001", "", "转介绍", "一对一在读", "电磁学薄弱、计算易错", "在读", "2026-11-25", "测试"}
	for i, v := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(sheet, cell, v)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	res, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Total != 1 || len(res.Rows) != 1 {
		t.Fatalf("total=%d rows=%d, want 1/1; issues=%v", res.Total, len(res.Rows), res.Issues)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("unexpected issues: %v", res.Issues)
	}
	row := res.Rows[0]
	if row.Name != "陈思远" || row.Phone != "13800000001" || row.GroupCode != "one_on_one" {
		t.Fatalf("unexpected row: %+v", row)
	}
	if row.NextFollowUp != "2026-11-25" {
		t.Fatalf("next follow up = %q", row.NextFollowUp)
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("Parse(nil) expected error")
	}
}
