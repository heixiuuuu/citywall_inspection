package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// pdfWriter 封装 gofpdf 的中文排版（mm 单位，手动换行与分页）。
type pdfWriter struct {
	pdf     *gofpdf.Fpdf
	left    float64
	pageW   float64
	pageBot float64
	y       float64
	imgSeq  int
}

const (
	pdfMargin  = 20.0 // mm
	lineHeight = 5.0  // mm
	sectionGap = 2.0
)

var reportLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// newPDF 创建 A4 文档并加载中文字体。
func newPDF(fontPath string) (*pdfWriter, error) {
	path, err := resolveFont(fontPath)
	if err != nil {
		return nil, err
	}
	p := gofpdf.New("P", "mm", "A4", "")
	fontData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p.AddUTF8FontFromBytes("hei", "", fontData)
	if err := p.Error(); err != nil {
		return nil, err
	}
	w := &pdfWriter{
		pdf:     p,
		left:    pdfMargin,
		pageW:   210.0,
		pageBot: 297.0 - 15.0,
		y:       pdfMargin,
	}
	w.pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	w.pdf.SetAutoPageBreak(false, 0)
	w.pdf.AddPage()
	return w, nil
}

// resolveFont 返回可用的中文字体路径。
func resolveFont(cfgPath string) (string, error) {
	candidates := []string{}
	if cfgPath != "" {
		candidates = append(candidates, cfgPath)
	}
	candidates = append(candidates,
		`/app/fonts/report.ttf`,
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\simsun.ttc`,
		`/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`,
		`/usr/share/fonts/truetype/wqy/wqy-microhei.ttc`,
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("report: no Chinese font found (set report.font_path)")
}

func (w *pdfWriter) ensure(h float64) {
	if w.y+h > w.pageBot {
		w.pdf.AddPage()
		w.y = pdfMargin
	}
	w.pdf.SetY(w.y)
}

// text 输出一行文本（自动分页）。
func (w *pdfWriter) text(s string, size float64, indent float64) {
	for _, line := range w.splitLines(s, size, w.pageW-2*pdfMargin-indent) {
		w.ensure(lineHeight)
		w.pdf.SetX(w.left + indent)
		w.pdf.CellFormat(0, lineHeight, line, "", 0, "L", false, 0, "")
		w.y += lineHeight
	}
}

func (w *pdfWriter) analysis(raw string) {
	var analysis struct {
		Status          string   `json:"status"`
		Model           string   `json:"model"`
		Description     string   `json:"description"`
		Causes          []string `json:"possible_causes"`
		Recommendations []string `json:"recommendations"`
	}
	if json.Unmarshal([]byte(raw), &analysis) != nil || analysis.Status != "SUCCEEDED" {
		return
	}
	w.text("视觉语言模型辅助分析（"+analysis.Model+"，须人工复核）："+analysis.Description, 9, 3)
	for _, cause := range analysis.Causes {
		w.text("可能成因："+cause, 9, 6)
	}
	for _, recommendation := range analysis.Recommendations {
		w.text("复查建议："+recommendation, 9, 6)
	}
}

// wrapText 输出多行文本（按宽度换行）。
func (w *pdfWriter) wrapText(s string, size float64, maxW float64) {
	w.pdf.SetFont("hei", "", size)
	lines := w.splitLines(s, size, maxW)
	for _, ln := range lines {
		w.ensure(lineHeight)
		w.pdf.SetX(w.left)
		w.pdf.CellFormat(0, lineHeight, ln, "", 0, "L", false, 0, "")
		w.pdf.Ln(lineHeight)
		w.y += lineHeight
	}
}

func (w *pdfWriter) splitLines(s string, size, maxW float64) []string {
	w.pdf.SetFont("hei", "", size)
	var lines []string
	var cur []rune
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, string(cur))
			cur = nil
			continue
		}
		t := string(append(append([]rune(nil), cur...), r))
		if w.pdf.GetStringWidth(t) > maxW && len(cur) > 0 {
			lines = append(lines, string(cur))
			cur = []rune{r}
		} else {
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 || len(lines) == 0 {
		lines = append(lines, string(cur))
	}
	return lines
}

// labelValue 同一行输出「标签：值」。
func (w *pdfWriter) labelValue(label, value string, labelSize float64, maxW float64) {
	w.text(label+" "+value, labelSize, 0)
}

// section 小节标题与分隔线。
func (w *pdfWriter) section(title string) {
	w.ensure(sectionGap + lineHeight + 4)
	w.y += sectionGap
	w.pdf.SetY(w.y)
	w.pdf.SetDrawColor(120, 120, 120)
	w.pdf.SetLineWidth(0.3)
	w.pdf.Line(w.left, w.y-1, w.pageW-pdfMargin, w.y-1)
	w.pdf.SetFont("hei", "", 13)
	w.pdf.SetX(w.left)
	w.pdf.CellFormat(0, lineHeight+4, title, "", 0, "L", false, 0, "")
	w.pdf.Ln(lineHeight + 4)
	w.y += lineHeight + 4
}

// title 文档大标题（居中）。
func (w *pdfWriter) title(s string) {
	w.ensure(10)
	w.pdf.SetFont("hei", "", 17)
	ww := w.pdf.GetStringWidth(s)
	w.pdf.SetX((w.pageW - ww) / 2)
	w.pdf.CellFormat(0, 9, s, "", 0, "L", false, 0, "")
	w.pdf.Ln(9)
	w.y += 9
}

// image 嵌入图片文件，按最大宽度等比缩放（mm 单位）。
func (w *pdfWriter) image(path, format string, pxW, pxH int, maxW float64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w.imgSeq++
	name := fmt.Sprintf("img%d", w.imgSeq)
	info := w.pdf.RegisterImageReader(name, format, f)
	if info == nil {
		return fmt.Errorf("image register failed: %s", path)
	}
	ratio := 0.0
	if info.Width() > 0 {
		ratio = info.Height() / info.Width()
	}
	hgt := maxW * ratio
	if hgt > 90 {
		hgt = 90
	}
	wdt := 1.0
	if ratio > 0 {
		wdt = hgt / ratio
	}
	w.ensure(hgt + 4)
	w.pdf.Image(name, w.left, w.y, wdt, hgt, false, format, 0, "")
	w.y += hgt + 4
	return nil
}

// renderOpts PDF 渲染上下文。
type renderOpts struct {
	GeneratedBy string
	SnapshotAt  string
	FontPath    string
	// OpenImage 打开 storage_key 对应的图片，返回本地临时文件路径与像素尺寸；
	// 调用方应在使用后执行 cleanup()。
	OpenImage func(key string) (path string, pxW, pxH int, mediaType string, cleanup func(), err error)
}

// imgFormat 由 media type 映射 gofpdf 图片类型。
func imgFormat(mediaType string) string {
	if mediaType == "image/png" {
		return "PNG"
	}
	if mediaType == "image/webp" {
		return "PNG"
	}
	return "JPG"
}

// renderTaskPDF 生成任务报告 PDF 字节。
func renderTaskPDF(src *TaskSource, opts renderOpts) ([]byte, error) {
	w, err := newPDF(opts.FontPath)
	if err != nil {
		return nil, err
	}
	w.title("古城墙病害巡检报告")
	w.text("任务 #"+itoa(src.TaskID)+" · "+src.Title, 10, 0)
	w.text("生成时间："+opts.SnapshotAt+"    生成人："+opts.GeneratedBy, 9, 0)

	w.section("任务信息")
	w.labelValue("负责人：", src.AssigneeName, 10, 500)
	w.labelValue("期限：", timeStr(src.DueAt), 10, 500)
	w.labelValue("状态：", src.Status, 10, 500)
	if src.DispatchedAt != nil {
		w.labelValue("派发时间：", timeStr(src.DispatchedAt), 10, 170)
	}
	if src.CompletedAt != nil {
		w.labelValue("完成时间：", timeStr(src.CompletedAt), 10, 170)
	}
	if src.Description != "" {
		w.wrapText("说明："+src.Description, 10, 170)
	}

	w.section("巡检汇总")
	w.text(fmt.Sprintf("点位总数 %d ｜ 已覆盖 %d ｜ 正常 %d ｜ 疑似病害 %d ｜ 批准未覆盖 %d ｜ 未覆盖 %d",
		src.PointTotal, src.PointCovered, src.PointNormal, src.PointSuspected, src.PointUnreachable, src.PointUncovered), 10, 0)
	w.text(fmt.Sprintf("人工判定误检 %d ｜ 人工补录（漏检）%d ｜ 重新识别次数 %d",
		src.RejectedCount, src.MissingCount, src.RetryCount), 10, 0)

	w.section("点位明细")
	for _, p := range src.Points {
		w.ensure(6)
		w.pdf.SetFont("hei", "", 11)
		w.pdf.SetX(w.left)
		w.pdf.CellFormat(0, 6, fmt.Sprintf("点位 %d ｜ %s（%s）", p.SequenceNo, p.PointName, p.PointCode), "", 0, "L", false, 0, "")
		w.pdf.Ln(lineHeight)
		w.y += lineHeight
		if p.SnapshotLocation != "" {
			w.text("快照位置："+p.SnapshotLocation, 9, 3)
		}
		if p.Observation == nil {
			w.text("本次未覆盖：无有效现场记录", 9, 3)
			continue
		}
		o := p.Observation
		w.text(fmt.Sprintf("记录 #%d ｜ 状态 %s ｜ 现场结论 %s ｜ 提交 %s",
			o.ID, o.Status, o.Outcome, timeStr(o.SubmittedAt)), 9, 3)
		if o.LocationAbnormal != "" {
			w.text("定位异常："+o.LocationAbnormal, 9, 3)
		}
		for _, a := range o.Attempts {
			w.analysis(a.AnalysisJSON)
			if len(a.Detections) == 0 {
				w.text(fmt.Sprintf("识别尝试 %d：%s（模型 %s，无候选）", a.AttemptNo, a.Status, a.ModelVersion), 9, 3)
				continue
			}
			w.text(fmt.Sprintf("识别尝试 %d：%s（模型 %s，候选 %d 个）", a.AttemptNo, a.Status, a.ModelVersion, len(a.Detections)), 9, 3)
			for _, d := range a.Detections {
				w.text(fmt.Sprintf("  候选 %s 置信度 %.3f 框(%d,%d,%d,%d)", d.ClassCode, d.Confidence, d.X1, d.Y1, d.X2, d.Y2), 9, 6)
			}
		}
		for _, c := range o.Conclusions {
			if len(c.Items) == 0 {
				w.text(fmt.Sprintf("人工结论 v%d：%s（%s）", c.RevisionNo, c.Outcome, c.Description), 9, 3)
				continue
			}
			w.text(fmt.Sprintf("人工结论 v%d：%s", c.RevisionNo, c.Outcome), 9, 3)
			for _, it := range c.Items {
				w.text(fmt.Sprintf("  图片 #%d %s → %s：%s", it.ImageID, it.Decision, it.FinalClassCode, it.Description), 9, 6)
			}
		}
		if o.Review != nil {
			w.text(fmt.Sprintf("审核：%s ｜ %s ｜ %s", o.Review.Decision, o.Review.Disposition, o.Review.Reason), 9, 3)
		}
		w.admission(o)
	}
	for _, point := range src.Points {
		if point.Observation != nil {
			w.photos(point.Observation.Images, opts)
		}
	}

	w.issues(src.Issues)
	w.text("", 9, 0)
	w.text("本报告为生成时刻数据快照，后续问题状态变化不影响本报告。", 8, 0)
	var buf bytes.Buffer
	if err := w.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderRangePDF 时间范围报告 PDF 字节。
func renderRangePDF(src *RangeSource, opts renderOpts) ([]byte, error) {
	w, err := newPDF(opts.FontPath)
	if err != nil {
		return nil, err
	}
	w.title("古城墙病害巡检报告（时间范围）")
	w.text(fmt.Sprintf("范围：%s ～ %s", src.Start.In(reportLocation).Format("2006-01-02 15:04"), src.End.In(reportLocation).Format("2006-01-02 15:04")), 10, 0)
	w.text("生成时间："+opts.SnapshotAt+"    生成人："+opts.GeneratedBy, 9, 0)

	w.section("汇总")
	w.text(fmt.Sprintf("记录总数 %d ｜ 提交 %d ｜ 通过 %d ｜ 退回 %d ｜ 驳回 %d",
		src.TotalRecords, src.SubmittedCount, src.AcceptedCount, src.ReturnedCount, src.RejectedCount), 10, 0)
	w.text(fmt.Sprintf("正常 %d ｜ 疑似病害 %d ｜ 无法到达 %d", src.NormalCount, src.SuspectedCount, src.UnreachableCount), 10, 0)

	w.section("记录明细")
	for idx, o := range src.Observations {
		w.ensure(6)
		w.pdf.SetFont("hei", "", 11)
		w.pdf.SetX(w.left)
		w.pdf.CellFormat(0, 6, fmt.Sprintf("记录 %d ｜ #%d ｜ %s ｜ %s", idx+1, o.ID, o.Source, o.LocationText), "", 0, "L", false, 0, "")
		w.pdf.Ln(lineHeight)
		w.y += lineHeight
		w.text(fmt.Sprintf("状态 %s ｜ 结论 %s ｜ 提交 %s", o.Status, o.Outcome, timeStr(o.SubmittedAt)), 9, 3)
		if o.LocationAbnormal != "" {
			w.text("定位异常："+o.LocationAbnormal, 9, 3)
		}
		for _, a := range o.Attempts {
			w.analysis(a.AnalysisJSON)
			w.text(fmt.Sprintf("识别 %d：%s（模型 %s，候选 %d）", a.AttemptNo, a.Status, a.ModelVersion, len(a.Detections)), 9, 3)
		}
		for _, c := range o.Conclusions {
			w.text(fmt.Sprintf("人工结论 v%d：%s（%d 项）", c.RevisionNo, c.Outcome, len(c.Items)), 9, 3)
		}
		if o.Review != nil {
			w.text(fmt.Sprintf("审核：%s ｜ %s", o.Review.Decision, o.Review.Reason), 9, 3)
		}
		w.admission(&o)
	}

	for _, observation := range src.Observations {
		w.photos(observation.Images, opts)
	}

	w.issues(src.Issues)
	w.text("", 9, 0)
	w.text("本报告为生成时刻数据快照，后续问题状态变化不影响本报告。", 8, 0)
	var buf bytes.Buffer
	if err := w.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (w *pdfWriter) issues(issues []IssueLinkView) {
	w.section("关联问题与处置进展")
	if len(issues) == 0 {
		w.text("无关联问题", 10, 0)
		return
	}
	statuses := map[string]string{"UNASSIGNED": "待分配", "OPEN": "已分配", "IN_PROGRESS": "处理中", "MONITORING": "观察中", "PENDING_VERIFICATION": "待核验", "CLOSED": "已关闭"}
	priorities := map[string]string{"WATCH": "关注", "PROCESS": "需处理", "URGENT": "尽快核实"}
	actions := map[string]string{"ASSIGN": "指派", "ADVICE": "处理建议", "PROGRESS": "处理进展", "OBSERVE": "观察记录", "REQUEST_VERIFY": "申请核验", "VERIFY_PASS": "核验通过", "VERIFY_RETURN": "核验退回", "REOPEN": "重新打开"}
	label := func(code string, names map[string]string) string {
		if name := names[code]; name != "" {
			return name
		}
		return code
	}
	for _, issue := range issues {
		heading := fmt.Sprintf("#%d %s ｜ 优先级 %s ｜ 状态 %s", issue.ID, issue.Title, label(issue.PriorityCode, priorities), label(issue.Status, statuses))
		w.ensure(float64(len(w.splitLines(heading, 10, w.pageW-2*pdfMargin))+1) * lineHeight)
		w.text(heading, 10, 0)
		assignee := issue.AssigneeName
		if assignee == "" {
			if issue.AssigneeID != nil {
				assignee = fmt.Sprintf("用户 #%d", *issue.AssigneeID)
			} else {
				assignee = "尚未指派"
			}
		}
		w.text("责任人："+assignee, 9, 3)
		if issue.ClosedAt != nil {
			w.text("关闭："+timeStr(issue.ClosedAt)+" ｜ "+issue.CloseReason, 9, 3)
		}
		if len(issue.Actions) == 0 {
			w.text("暂无处置记录", 9, 3)
		}
		for _, action := range issue.Actions {
			actor := action.ActorName
			if actor == "" {
				actor = fmt.Sprintf("用户 #%d", action.ActorID)
			}
			heading := timeStr(&action.CreatedAt) + " ｜ " + label(action.ActionType, actions) + " ｜ " + actor
			w.ensure(float64(len(w.splitLines(heading, 9, w.pageW-2*pdfMargin-3))+1) * lineHeight)
			w.text(heading, 9, 3)
			w.text(action.Content, 9, 6)
			if action.PlannedFollowUpAt != nil {
				w.text("计划复查："+timeStr(action.PlannedFollowUpAt), 9, 6)
			}
		}
	}
}

func (w *pdfWriter) admission(o *ObservationView) {
	if o.AdmittedAt == nil {
		if o.Status == "ACCEPTED" {
			w.text("受理：尚未受理", 9, 3)
		}
		return
	}
	result := "已立项或关联问题"
	if o.NoIssueReason != "" {
		result = "不立项：" + o.NoIssueReason
	}
	w.text("受理："+result+" ｜ "+timeStr(o.AdmittedAt), 9, 3)
}

func timeStr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.In(reportLocation).Format("2006-01-02 15:04:05")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
