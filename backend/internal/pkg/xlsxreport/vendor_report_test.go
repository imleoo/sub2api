package xlsxreport

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/statement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func sampleStatement() *statement.Statement {
	return &statement.Statement{
		Period:         "2026-05",
		Timezone:       "UTC",
		Source:         "snapshot",
		Closed:         true,
		OpeningBalance: 1000,
		ClosingBalance: 4410,
		UserEmail:      "u@test.com",
		Rows: []statement.Row{
			{Date: "2026-05-01", Nature: statement.NatureOpening, RunningTotal: 1000},
			{Date: "2026-05-01", Nature: statement.NatureDeposit, Amount: 5000, Note: "order A", RunningTotal: 6000},
			{Date: "2026-05-02", Nature: statement.NatureUtilisation, Qty: -20000, UnitPrice: 0.05, CostBefore: 1000, DiscountRate: 0.4, CostAfter: 600, RunningTotal: 5400},
			{Date: "2026-05-04", Nature: statement.NatureWithdraw, Amount: 1000, Note: "refund", RunningTotal: 4400},
			{Date: "2026-05-06", Nature: statement.NatureCredit, Amount: 10, Note: "redeem", RunningTotal: 4410},
			{Date: "2026-05-31", Nature: statement.NatureClosing, RunningTotal: 4410},
		},
		Totals: statement.Totals{Deposit: 5000, Withdraw: 1000, Credit: 10, UtilisationBefore: 1000, UtilisationAfter: 600},
	}
}

func TestBuildVendorReport_TemplateLayoutAndFormulas(t *testing.T) {
	buf, err := BuildVendorReport(sampleStatement())
	require.NoError(t, err)

	f, err := excelize.OpenReader(buf)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	get := func(cell string) string {
		// RawCellValue：断言存储值而非数字格式化后的显示值。
		v, err := f.GetCellValue(vendorReportSheet, cell, excelize.Options{RawCellValue: true})
		require.NoError(t, err)
		return v
	}
	getFormula := func(cell string) string {
		v, err := f.GetCellFormula(vendorReportSheet, cell)
		require.NoError(t, err)
		return v
	}

	// 标题区与列组说明。
	assert.Equal(t, "Vendor report", get("A1"))
	assert.Equal(t, "Period: May 2026", get("A2"))
	assert.Equal(t, "C = A x B", get("G3"))

	// 双行表头。
	assert.Equal(t, "Date", get("A4"))
	assert.Equal(t, "Deposit/(Withdraw)", get("C4"))
	assert.Equal(t, "Discount rate", get("H4"))
	assert.Equal(t, "Unit Price (USD)", get("F5"))
	assert.Equal(t, "Qty", get("E5"))

	// 期初行（模板行 6）。
	assert.Equal(t, "A/C Opening balance", get("B6"))
	assert.Equal(t, "1000", get("J6"))

	// Deposit 行（行 7）：J=C+I 活公式。
	assert.Equal(t, "Deposit", get("B7"))
	assert.Equal(t, "5000", get("C7"))
	assert.Equal(t, "C7+I7", getFormula("J7"))

	// Utilisation 行（行 8）：A×B=C、C×(1−D) 活公式，Qty 负数。
	assert.Equal(t, "Utilisation", get("B8"))
	assert.Equal(t, "-20000", get("E8"))
	assert.Equal(t, "E8*F8", getFormula("G8"))
	assert.Equal(t, "G8*(1-H8)", getFormula("I8"))
	assert.Equal(t, "C8+I8", getFormula("J8"))

	// Withdraw 行（行 9）：C 列写负数。
	assert.Equal(t, "Withdraw", get("B9"))
	assert.Equal(t, "-1000", get("C9"))

	// Credit 行（行 10）：J=D+I。
	assert.Equal(t, "Credit", get("B10"))
	assert.Equal(t, "10", get("D10"))
	assert.Equal(t, "D10+I10", getFormula("J10"))

	// 期末行（行 11）：SUM 公式（数据行 7-10，J 列含期初行 6）。
	assert.Equal(t, "A/C Closing balance", get("B11"))
	assert.Equal(t, "SUM(C7:C10)", getFormula("C11"))
	assert.Equal(t, "SUM(D7:D10)", getFormula("D11"))
	assert.Equal(t, "SUM(I7:I10)", getFormula("I11"))
	assert.Equal(t, "SUM(J6:J10)", getFormula("J11"))
}

func TestBuildVendorReport_PartialMonthNote(t *testing.T) {
	st := sampleStatement()
	st.Closed = false
	st.Totals.IdentityGap = 1.5

	buf, err := BuildVendorReport(st)
	require.NoError(t, err)
	f, err := excelize.OpenReader(buf)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(vendorReportSheet)
	require.NoError(t, err)
	var flat string
	for _, r := range rows {
		for _, c := range r {
			flat += c + "\n"
		}
	}
	assert.Contains(t, flat, "not yet closed")
	assert.Contains(t, flat, "identity gap")
}

func TestBuildVendorReport_NilAndBadPeriod(t *testing.T) {
	_, err := BuildVendorReport(nil)
	assert.Error(t, err)
	_, err = BuildVendorReport(&statement.Statement{Period: "bogus"})
	assert.Error(t, err)
}
