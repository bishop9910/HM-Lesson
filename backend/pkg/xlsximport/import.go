// Package xlsximport 解析学员 Excel 导入文件并生成模板。
// 模板列与飞书客户档案表对齐；家长飞书账号列保留但不读取。
package xlsximport

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Row 是导入文件中的一行；NextFollowUp 统一为 YYYY-MM-DD。
type Row struct {
	Row          int    `json:"row"`
	Name         string `json:"name"`
	Grade        string `json:"grade"`
	School       string `json:"school"`
	ParentName   string `json:"parent_name"`
	Phone        string `json:"phone"`
	Source       string `json:"source"`
	GroupCode    string `json:"group_code"`
	Tags         string `json:"tags"`
	Status       string `json:"status"`
	NextFollowUp string `json:"next_follow_up"`
	Note         string `json:"note"`
}

// Issue 是一条行级校验错误。
type Issue struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Result 是解析结果。
type Result struct {
	Total  int
	Rows   []Row
	Issues []Issue
}

var headerAliases = map[string]string{
	"学员姓名": "name", "姓名": "name",
	"年级":    "grade",
	"学校":    "school",
	"家长姓名":  "parent_name",
	"家长手机号": "phone", "家长手机": "phone", "手机号": "phone",
	"来源渠道": "source", "来源": "source",
	"客户分组": "group", "分组": "group",
	"自定义标签": "tags", "标签": "tags",
	"当前状态": "status", "状态": "status",
	"下次回访时间": "next_follow_up", "下次回访": "next_follow_up",
	"备注": "note",
	// 飞书账号列只占位，不读取。
	"家长飞书账号": "ignore",
}

// Template 生成只有表头的导入模板。
func Template() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "学员导入"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"学员姓名", "年级", "学校", "家长姓名", "家长手机号", "家长飞书账号", "来源渠道", "客户分组", "自定义标签", "当前状态", "下次回访时间", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	_ = f.SetCellStyle(sheet, "A1", "L1", style)
	_ = f.SetColWidth(sheet, "A", "A", 14)
	_ = f.SetColWidth(sheet, "B", "C", 12)
	_ = f.SetColWidth(sheet, "D", "F", 16)
	_ = f.SetColWidth(sheet, "G", "J", 14)
	_ = f.SetColWidth(sheet, "K", "K", 14)
	_ = f.SetColWidth(sheet, "L", "L", 28)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Parse 解析第一张工作表；首行为表头。
func Parse(data []byte) (Result, error) {
	var result Result
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return result, fmt.Errorf("无法读取 Excel：%w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return result, fmt.Errorf("Excel 中没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return result, fmt.Errorf("读取工作表失败：%w", err)
	}
	if len(rows) == 0 {
		return result, fmt.Errorf("工作表为空")
	}

	cols := make(map[string]int)
	for i, raw := range rows[0] {
		key, ok := headerAliases[strings.TrimSpace(raw)]
		if ok && key != "ignore" {
			cols[key] = i
		}
	}
	if _, ok := cols["name"]; !ok {
		return result, fmt.Errorf("缺少“学员姓名”列")
	}

	for i := 1; i < len(rows); i++ {
		raw := rows[i]
		if emptyRow(raw) {
			continue
		}
		result.Total++
		rowNum := i + 1
		get := func(key string) string {
			idx, ok := cols[key]
			if !ok || idx >= len(raw) {
				return ""
			}
			return strings.TrimSpace(raw[idx])
		}
		row := Row{
			Row:        rowNum,
			Name:       get("name"),
			Grade:      get("grade"),
			School:     get("school"),
			ParentName: get("parent_name"),
			Phone:      normalizePhone(get("phone")),
			Source:     get("source"),
			GroupCode:  normalizeGroup(get("group")),
			Tags:       normalizeTags(get("tags")),
			Status:     normalizeStatus(get("status")),
			Note:       get("note"),
		}
		if row.NextFollowUp, err = normalizeDate(get("next_follow_up")); err != nil {
			result.Issues = append(result.Issues, Issue{Row: rowNum, Field: "下次回访时间", Message: err.Error()})
		}
		if row.Name == "" {
			result.Issues = append(result.Issues, Issue{Row: rowNum, Field: "学员姓名", Message: "不能为空"})
			continue
		}
		if row.Phone == "" {
			result.Issues = append(result.Issues, Issue{Row: rowNum, Field: "家长手机号", Message: "不能为空"})
			continue
		}
		if len(row.Phone) != 11 {
			result.Issues = append(result.Issues, Issue{Row: rowNum, Field: "家长手机号", Message: "应为 11 位手机号"})
		}
		if len(result.Issues) > 0 && hasIssue(result.Issues, rowNum) {
			continue
		}
		result.Rows = append(result.Rows, row)
	}
	if result.Total > 5000 {
		return result, fmt.Errorf("单次最多导入 5000 行，当前 %d 行", result.Total)
	}
	return result, nil
}

func emptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func hasIssue(issues []Issue, row int) bool {
	for _, issue := range issues {
		if issue.Row == row {
			return true
		}
	}
	return false
}

func normalizePhone(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, "-", "")
	if strings.HasPrefix(v, "+86") {
		v = strings.TrimPrefix(v, "+86")
	}
	if strings.HasPrefix(v, "86") && len(v) == 13 {
		v = v[2:]
	}
	return v
}

func normalizeGroup(v string) string {
	switch strings.TrimSpace(v) {
	case "班课在读", "班课":
		return "class_active"
	case "一对一在读", "一对一", "1v1":
		return "one_on_one"
	case "待转化意向", "待转化":
		return "intent"
	case "咨询访客", "访客":
		return "visitor"
	case "class_active", "one_on_one", "intent", "visitor":
		return v
	default:
		return "visitor"
	}
}

func normalizeStatus(v string) string {
	switch strings.TrimSpace(v) {
	case "在读", "active", "":
		return "active"
	case "结课归档", "结课", "graduated":
		return "graduated"
	case "流失", "lost":
		return "lost"
	default:
		return "active"
	}
}

func normalizeTags(v string) string {
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if s := strings.TrimSpace(f); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, ",")
}

func normalizeDate(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	layouts := []string{"2006-01-02", "2006/01/02", "2006/1/2", "2006-1-2", "2006-01-02 15:04:05", "2006/1/2 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	// Excel 日期序列号。
	if serial, err := strconv.ParseFloat(v, 64); err == nil && serial > 0 {
		if t, err := excelize.ExcelDateToTime(serial, false); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("应为 YYYY-MM-DD 格式")
}
