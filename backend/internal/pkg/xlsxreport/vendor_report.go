// Package xlsxreport 用 excelize 生成 Eonreach Vendor Report 格式的月度对账单 Excel。
// 布局逐格复刻模板 docs/需求/Vendor report template - Eonreach (1).xlsx：
// 标题 / Period / A×B=C 列组说明 / 双行表头 / 六种行 / 期末 SUM 行，
// 金额列写活公式（=E8*F8、=G8*(1-H8)、=C8+I8、=SUM(...)）保持可审计性。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
package xlsxreport

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/statement"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const vendorReportSheet = "Sheet1"

// VendorReportMIME 是 xlsx 附件的 Content-Type。
const VendorReportMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// WriteVendorReportAttachment 渲染对账单并作为 xlsx 附件写出（用户端与管理端
// 共用的唯一出口，文件名/头部口径集中于此；header 模式与 redeem CSV 导出一致）。
// 未封账月文件名带 -partial 后缀。
func WriteVendorReportAttachment(c *gin.Context, st *statement.Statement) {
	buf, err := BuildVendorReport(st)
	if err != nil {
		response.InternalError(c, "Failed to build statement report")
		return
	}
	suffix := ""
	if !st.Closed {
		suffix = "-partial"
	}
	filename := fmt.Sprintf("statement-%s-%s%s.xlsx", st.UserEmail, st.Period, suffix)
	c.Header("Content-Type", VendorReportMIME)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, VendorReportMIME, buf.Bytes())
}

// BuildVendorReport 把对账单渲染为 xlsx 字节流。
func BuildVendorReport(st *statement.Statement) (*bytes.Buffer, error) {
	if st == nil {
		return nil, fmt.Errorf("nil statement")
	}
	monthStart, err := time.Parse("2006-01", st.Period)
	if err != nil {
		return nil, fmt.Errorf("invalid period %q: %w", st.Period, err)
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := vendorReportSheet

	set := func(cell string, value any) {
		_ = f.SetCellValue(sheet, cell, value)
	}
	formula := func(cell, expr string) {
		// excelize 惯例：公式不带前导 =，读回（GetCellFormula）亦如此。
		_ = f.SetCellFormula(sheet, cell, strings.TrimPrefix(expr, "="))
	}

	// 标题区（模板行 1-2）。
	set("A1", "Vendor report")
	set("A2", "Period: "+monthStart.Format("January 2006"))

	// 列组说明（模板行 3）：E=A、F=B、G=C=A×B、H=D、I=C×D。
	set("E3", "A")
	set("F3", "B")
	set("G3", "C = A x B")
	set("H3", "D")
	set("I3", "C x D")

	// 双行表头（模板行 4-5）。
	headers := map[string]string{
		"A4": "Date", "B4": "Nature", "C4": "Deposit/(Withdraw)", "D4": "Credit",
		"E4": "Cost (before discount)", "F4": "Cost (before discount)", "G4": "Cost (before discount)",
		"H4": "Discount rate", "I4": "Cost (after discount)", "J4": "Total",
	}
	units := map[string]string{
		"C5": "USD", "D5": "USD", "E5": "Qty", "F5": "Unit Price (USD)",
		"G5": "USD", "H5": "%", "I5": "USD", "J5": "USD",
	}
	for cell, v := range headers {
		set(cell, v)
	}
	for cell, v := range units {
		set(cell, v)
	}

	dateStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 14}) // m/d/yyyy
	moneyStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: new("#,##0.00")})
	costStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: new("#,##0.000000")})
	pctStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 10}) // 0.00%

	writeDate := func(row int, date string) {
		if d, err := time.Parse("2006-01-02", date); err == nil {
			set(fmt.Sprintf("A%d", row), d)
			_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), dateStyle)
		} else {
			set(fmt.Sprintf("A%d", row), date)
		}
	}

	row := 6
	firstDataRow := 7 // 模板：期初行后第一条流水行
	var lastFlowRow int

	for _, r := range st.Rows {
		switch r.Nature {
		case statement.NatureOpening:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "A/C Opening balance")
			set(fmt.Sprintf("J%d", row), r.RunningTotal)
		case statement.NatureDeposit:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "Deposit")
			set(fmt.Sprintf("C%d", row), r.Amount)
			formula(fmt.Sprintf("J%d", row), fmt.Sprintf("=C%d+I%d", row, row))
		case statement.NatureWithdraw:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "Withdraw")
			set(fmt.Sprintf("C%d", row), -r.Amount)
			formula(fmt.Sprintf("J%d", row), fmt.Sprintf("=C%d+I%d", row, row))
		case statement.NatureCredit:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "Credit")
			set(fmt.Sprintf("D%d", row), r.Amount)
			formula(fmt.Sprintf("J%d", row), fmt.Sprintf("=D%d+I%d", row, row))
		case statement.NatureUtilisation:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "Utilisation")
			set(fmt.Sprintf("E%d", row), r.Qty)
			set(fmt.Sprintf("F%d", row), r.UnitPrice)
			formula(fmt.Sprintf("G%d", row), fmt.Sprintf("=E%d*F%d", row, row))
			set(fmt.Sprintf("H%d", row), r.DiscountRate)
			formula(fmt.Sprintf("I%d", row), fmt.Sprintf("=G%d*(1-H%d)", row, row))
			formula(fmt.Sprintf("J%d", row), fmt.Sprintf("=C%d+I%d", row, row))
			_ = f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), pctStyle)
		case statement.NatureClosing:
			writeDate(row, r.Date)
			set(fmt.Sprintf("B%d", row), "A/C Closing balance")
			if lastFlowRow >= firstDataRow {
				formula(fmt.Sprintf("C%d", row), fmt.Sprintf("=SUM(C%d:C%d)", firstDataRow, lastFlowRow))
				formula(fmt.Sprintf("D%d", row), fmt.Sprintf("=SUM(D%d:D%d)", firstDataRow, lastFlowRow))
				formula(fmt.Sprintf("I%d", row), fmt.Sprintf("=SUM(I%d:I%d)", firstDataRow, lastFlowRow))
			}
			// J 列自期初行(6)起求和 = 期初 + 全部行净变动 = 期末。
			formula(fmt.Sprintf("J%d", row), fmt.Sprintf("=SUM(J6:J%d)", row-1))
		}
		if r.Nature != statement.NatureOpening && r.Nature != statement.NatureClosing {
			lastFlowRow = row
		}
		row++
	}

	// 金额区列样式（C/D/G/I/J 金额、E 数量、F 单价高精度）。
	lastRow := row - 1
	for _, col := range []string{"C", "D", "G", "I", "J"} {
		_ = f.SetCellStyle(sheet, col+"6", fmt.Sprintf("%s%d", col, lastRow), moneyStyle)
	}
	_ = f.SetCellStyle(sheet, "F6", fmt.Sprintf("F%d", lastRow), costStyle)
	_ = f.SetColWidth(sheet, "A", "B", 18)
	_ = f.SetColWidth(sheet, "C", "J", 14)

	// 未封账提示。
	if !st.Closed {
		set(fmt.Sprintf("A%d", lastRow+2), "Note: current month, not yet closed; figures may still change.")
	}
	if st.Totals.IdentityGap != 0 {
		set(fmt.Sprintf("A%d", lastRow+3), fmt.Sprintf("Note: identity gap %.8f USD (untracked balance adjustments).", st.Totals.IdentityGap))
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return &buf, nil
}
