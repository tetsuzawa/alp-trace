package stats

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/tetsuzawa/alp-trace/helpers"
	"github.com/tetsuzawa/alp-trace/html"
)

func traceKeywords(percentiles []int) []string {
	s1 := []string{
		"count",
		"uri_method_status",
		"min",
		"max",
		"sum",
		"avg",
	}

	s2 := []string{
		"stddev",
		"min_body",
		"max_body",
		"sum_body",
		"avg_body",
	}

	sp := make([]string, 0, len(percentiles))
	for _, p := range percentiles {
		sp = append(sp, fmt.Sprintf("p%d", p))
	}

	ss := []string{
		"trace_id_sample",
	}

	s := make([]string, 0, len(s1)+len(s2)+len(sp)+len(ss))
	s = append(s, s1...)
	s = append(s, sp...)
	s = append(s, s2...)
	s = append(s, ss...)

	return s
}

func traceDefaultHeaders(percentiles []int) []string {
	s1 := []string{
		"Count",
		"UriMethodStatus",
		"Min",
		"Max",
		"Sum",
		"Avg",
	}

	s2 := []string{
		"Stddev",
		"Min(Body)",
		"Max(Body)",
		"Sum(Body)",
		"Avg(Body)",
	}

	sp := make([]string, 0, len(percentiles))
	for _, p := range percentiles {
		sp = append(sp, fmt.Sprintf("P%d", p))
	}

	ss := []string{
		"TraceIdSample",
	}

	s := make([]string, 0, len(s1)+len(s2)+len(sp)+len(ss))
	s = append(s, s1...)
	s = append(s, sp...)
	s = append(s, s2...)
	s = append(s, ss...)

	return s
}

func traceHeadersMap(percentiles []int) map[string]string {
	headers := map[string]string{
		"count":             "Count",
		"uri_method_status": "UriMethodStatus",
		"min":               "Min",
		"max":               "Max",
		"sum":               "Sum",
		"avg":               "Avg",
		"stddev":            "Stddev",
		"min_body":          "Min(Body)",
		"max_body":          "Max(Body)",
		"sum_body":          "Sum(Body)",
		"avg_body":          "Avg(Body)",
		"trace_id_sample":   "TraceIdSample",
	}

	for _, p := range percentiles {
		key := fmt.Sprintf("p%d", p)
		val := fmt.Sprintf("P%d", p)
		headers[key] = val
	}

	return headers
}

type TracePrintOptions struct {
	noHeaders       bool
	showFooters     bool
	decodeUri       bool
	paginationLimit int
}

func NewTracePrintOptions(noHeaders, showFooters, decodeUri bool, paginationLimit int) *TracePrintOptions {
	return &TracePrintOptions{
		noHeaders:       noHeaders,
		showFooters:     showFooters,
		decodeUri:       decodeUri,
		paginationLimit: paginationLimit,
	}
}

type TracePrinter struct {
	keywords     []string
	format       string
	percentiles  []int
	printOptions *TracePrintOptions
	headers      []string
	headersMap   map[string]string
	writer       io.Writer
	all          bool
}

func NewTracePrinter(w io.Writer, val, format string, percentiles []int, printOptions *TracePrintOptions) *TracePrinter {
	p := &TracePrinter{
		format:       format,
		percentiles:  percentiles,
		headersMap:   traceHeadersMap(percentiles),
		writer:       w,
		printOptions: printOptions,
	}

	if val == "all" {
		p.keywords = traceKeywords(percentiles)
		p.headers = traceDefaultHeaders(percentiles)
		p.all = true
	} else {
		p.keywords = helpers.SplitCSV(val)
		for _, key := range p.keywords {
			p.headers = append(p.headers, p.headersMap[key])
			if key == "all" {
				p.keywords = traceKeywords(percentiles)
				p.headers = traceDefaultHeaders(percentiles)
				p.all = true
				break
			}
		}
	}

	return p
}

func (p *TracePrinter) Validate() error {
	if p.all {
		return nil
	}

	invalids := make([]string, 0)
	for _, key := range p.keywords {
		if _, ok := p.headersMap[key]; !ok {
			invalids = append(invalids, key)
		}
	}

	if len(invalids) > 0 {
		return fmt.Errorf("invalid keywords: %s", strings.Join(invalids, ","))
	}

	return nil
}

func (p *TracePrinter) GenerateTraceLine(s *ScenarioStat, quoteUri bool) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, s.StrCount())
		case "uri_method_status":
			uriMethodStatus := s.UriWithOptions(p.printOptions.decodeUri)
			if quoteUri && strings.Contains(s.TraceUriMethodStatus, ",") {
				uriMethodStatus = fmt.Sprintf(`"%s"`, s.TraceUriMethodStatus)
			}
			line = append(line, uriMethodStatus)
		case "min":
			line = append(line, round(s.MinResponseTime()))
		case "max":
			line = append(line, round(s.MaxResponseTime()))
		case "sum":
			line = append(line, round(s.SumResponseTime()))
		case "avg":
			line = append(line, round(s.AvgResponseTime()))
		case "stddev":
			line = append(line, round(s.StddevResponseTime()))
		case "min_body":
			line = append(line, round(s.MinResponseBodyBytes()))
		case "max_body":
			line = append(line, round(s.MaxResponseBodyBytes()))
		case "sum_body":
			line = append(line, round(s.SumResponseBodyBytes()))
		case "avg_body":
			line = append(line, round(s.AvgResponseBodyBytes()))
		case "trace_id_sample":
			traceIDSample := s.RandomTraceID()
			line = append(line, traceIDSample)
		default: // percentile
			var n int
			_, err := fmt.Sscanf(p.keywords[i], "p%d", &n)
			if err != nil {
				continue
			}
			line = append(line, round(s.PNResponseTime(n)))
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceLineWithDiff(from, to *ScenarioStat, quoteUri bool) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	differ := NewTraceDiffer(from, to)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, formattedLineWithDiff(to.StrCount(), differ.DiffCnt()))
		case "uri_method_status":
			uriMethodStatus := to.UriWithOptions(p.printOptions.decodeUri)
			if quoteUri && strings.Contains(to.TraceUriMethodStatus, ",") {
				uriMethodStatus = fmt.Sprintf(`"%s"`, to.TraceUriMethodStatus)
			}
			line = append(line, uriMethodStatus)
		case "min":
			line = append(line, formattedLineWithDiff(round(to.MinResponseTime()), differ.DiffMinResponseTime()))
		case "max":
			line = append(line, formattedLineWithDiff(round(to.MaxResponseTime()), differ.DiffMaxResponseTime()))
		case "sum":
			line = append(line, formattedLineWithDiff(round(to.SumResponseTime()), differ.DiffSumResponseTime()))
		case "avg":
			line = append(line, formattedLineWithDiff(round(to.AvgResponseTime()), differ.DiffAvgResponseTime()))
		case "stddev":
			line = append(line, formattedLineWithDiff(round(to.StddevResponseTime()), differ.DiffStddevResponseTime()))
		case "min_body":
			line = append(line, formattedLineWithDiff(round(to.MinResponseBodyBytes()), differ.DiffMinResponseBodyBytes()))
		case "max_body":
			line = append(line, formattedLineWithDiff(round(to.MaxResponseBodyBytes()), differ.DiffMaxResponseBodyBytes()))
		case "sum_body":
			line = append(line, formattedLineWithDiff(round(to.SumResponseBodyBytes()), differ.DiffSumResponseBodyBytes()))
		case "avg_body":
			line = append(line, formattedLineWithDiff(round(to.AvgResponseBodyBytes()), differ.DiffAvgResponseBodyBytes()))
		default: // percentile
			var n int
			_, err := fmt.Sscanf(p.keywords[i], "p%d", &n)
			if err != nil {
				continue
			}
			line = append(line, formattedLineWithDiff(round(to.PNResponseTime(n)), differ.DiffPNResponseTime(n)))
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceFooter(counts map[string]int) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, fmt.Sprint(counts["count"]))
		default:
			line = append(line, "")
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceFooterWithDiff(countsFrom, countsTo map[string]int) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)
	counts := DiffCountAll(countsFrom, countsTo)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, formattedLineWithDiff(fmt.Sprint(countsTo["count"]), counts["count"]))
		default:
			line = append(line, "")
		}
	}

	return line
}

func (p *TracePrinter) SetFormat(format string) {
	p.format = format
}

func (p *TracePrinter) SetHeaders(headers []string) {
	p.headers = headers
}

func (p *TracePrinter) SetWriter(w io.Writer) {
	p.writer = w
}

func (p *TracePrinter) Print(ts, tsTo *TraceStats) {
	switch p.format {
	case "pretty":
		p.printTracePretty(ts, tsTo)
	case "table":
		p.printTraceTable(ts, tsTo)
	case "md", "markdown":
		p.printTraceMarkdown(ts, tsTo)
	case "tsv":
		p.printTraceTSV(ts, tsTo)
	case "csv":
		p.printTraceCSV(ts, tsTo)
	case "html":
		p.printTraceHTML(ts, tsTo)
	}
}

//func round(num float64) string {
//	return fmt.Sprintf("%.3f", num)
//}

func findTraceStatFrom(tsFrom *TraceStats, tsTo *ScenarioStat) *ScenarioStat {
	for _, sFrom := range tsFrom.ScenarioStats {
		if sFrom.TraceUriMethodStatus == tsTo.TraceUriMethodStatus {
			return sFrom
		}
	}
	return nil
}

func (p *TracePrinter) printTracePretty(tsFrom, tsTo *TraceStats) {
	funcMap := template.FuncMap{
		"currentDate": func() string {
			return time.Now().Local().Format(time.RFC3339)
		},
		"percent": func(a, b interface{}) float64 {
			ta := anyToFloat64(a)
			tb := anyToFloat64(b)
			return ta / tb * 100
		},
		"per": func(a, b interface{}) float64 {
			ta := anyToFloat64(a)
			tb := anyToFloat64(b)
			return ta / tb
		},
		"rank": func(a int) int {
			return a + 1
		},
		"shortTime": func(v interface{}) string {
			var format string
			f := anyToFloat64(v)
			if f < 0.000000001 {
				format = "%.0f"
			} else if f < 0.000001 {
				f = f * 1000000000
				format = "%.1fns"
			} else if f < 0.001 {
				f = f * 1000000
				format = "%.1fus"
			} else if f < 1 {
				f = f * 1000
				format = "%.1fms"
			} else {
				format = "%.2fs"
			}
			return fmt.Sprintf(format, f)
		},
		"shortByteInt": func(v interface{}) string {
			var format string
			f := anyToFloat64(v)
			if f >= 1024*1024*1024 {
				f = f / (1024 * 1024 * 1024)
				format = "%.0fG"
			} else if f >= 1024*1024 {
				f = f / (1024 * 1024)
				format = "%.0fM"
			} else if f >= 1024 {
				f = f / 1024
				format = "%.0fk"
			} else {
				format = "%.0f"
			}
			return fmt.Sprintf(format, f)
		},
		"shortByte": func(v interface{}) string {
			var format string
			f := anyToFloat64(v)
			if f >= 1024*1024*1024 {
				f = f / (1024 * 1024 * 1024)
				format = "%.2fG"
			} else if f >= 1024*1024 {
				f = f / (1024 * 1024)
				format = "%.2fM"
			} else if f >= 1024 {
				f = f / 1024
				format = "%.2fk"
			} else if f == 0 {
				format = "%.0f"
			} else {
				format = "%.2f"
			}
			return fmt.Sprintf(format, f)
		},
		"shortInt": func(v interface{}) string {
			var format string
			f := anyToFloat64(v)
			if f >= 1_000_000_000 {
				f = f / 1_000_000_000
				format = "%.2fG"
			} else if f >= 1_000_000 {
				f = f / 1_000_000
				format = "%.2fM"
			} else if f >= 1_000 {
				f = f / 1_000
				format = "%.2fk"
			} else {
				format = "%.0f"
			}
			return fmt.Sprintf(format, f)
		},
		"short": func(v interface{}) string {
			var format string
			f := anyToFloat64(v)
			if f >= 1_000_000_000 {
				f = f / 1_000_000_000
				format = "%.2fG"
			} else if f >= 1_000_000 {
				f = f / 1_000_000
				format = "%.2fM"
			} else if f >= 1_000 {
				f = f / 1_000
				format = "%.2fk"
			} else if f == 0 {
				format = "%.0f"
			} else {
				format = "%.2f"
			}
			return fmt.Sprintf(format, f)
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(fs, "templates/pretty.tmpl")
	if err != nil {
		fmt.Println(err)
	}
	for _, tpl := range tmpl.Templates() {
		fmt.Println(tpl.Name())
	}
	if tsTo == nil {
		err := tmpl.ExecuteTemplate(os.Stdout, "pretty.tmpl", tsFrom)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				err := tmpl.ExecuteTemplate(os.Stdout, "pretty.tmpl", tsTo)
				if err != nil {
					fmt.Println(err)
				}
			} else {
				err := tmpl.ExecuteTemplate(os.Stdout, "pretty.tmpl", tsTo)
				if err != nil {
					fmt.Println(err)
				}
				//return tmpl.ExecuteTemplateWithDiff(w, "report_diff.tmpl", result)
			}
		}
	}

}

func (p *TracePrinter) printTraceTable(tsFrom, tsTo *TraceStats) {
	table := tablewriter.NewWriter(p.writer)
	table.SetAutoWrapText(false)
	table.SetHeader(p.headers)
	if tsTo == nil {
		for _, s := range tsFrom.ScenarioStats {
			data := p.GenerateTraceLine(s, false)
			table.Append(data)
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			var data []string
			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			table.Append(data)
		}
	}

	if p.printOptions.showFooters {
		var footer []string
		if tsTo == nil {
			footer = p.GenerateTraceFooter(tsFrom.CountAll())
		} else {
			footer = p.GenerateTraceFooterWithDiff(tsFrom.CountAll(), tsTo.CountAll())
		}
		table.SetFooter(footer)
		table.SetFooterAlignment(tablewriter.ALIGN_LEFT)
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
}

func (p *TracePrinter) printTraceMarkdown(tsFrom, tsTo *TraceStats) {
	table := tablewriter.NewWriter(p.writer)
	table.SetHeader(p.headers)
	table.SetAutoWrapText(false)
	table.SetAutoMergeCells(true)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	if tsTo == nil {
		for _, s := range tsFrom.ScenarioStats {
			data := p.GenerateTraceLine(s, false)
			table.Append(data)
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			var data []string
			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			table.Append(data)
		}
	}

	if p.printOptions.showFooters {
		var footer []string
		if tsTo == nil {
			footer = p.GenerateTraceFooter(tsFrom.CountAll())
		} else {
			footer = p.GenerateTraceFooterWithDiff(tsFrom.CountAll(), tsTo.CountAll())
		}
		table.Append(footer)
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
}

func (p *TracePrinter) printTraceTSV(tsFrom, tsTo *TraceStats) {
	if !p.printOptions.noHeaders {
		fmt.Println(strings.Join(p.headers, "\t"))
	}

	var data []string
	if tsTo == nil {
		for _, s := range tsFrom.ScenarioStats {
			data = p.GenerateTraceLine(s, false)
			fmt.Println(strings.Join(data, "\t"))
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			fmt.Println(strings.Join(data, "\t"))
		}
	}
}

func (p *TracePrinter) printTraceCSV(tsFrom, tsTo *TraceStats) {
	if !p.printOptions.noHeaders {
		fmt.Println(strings.Join(p.headers, ","))
	}

	var data []string
	if tsTo == nil {
		for _, s := range tsFrom.ScenarioStats {
			data = p.GenerateTraceLine(s, true)
			fmt.Println(strings.Join(data, ","))
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			fmt.Println(strings.Join(data, ","))
		}
	}
}

func (p *TracePrinter) printTraceHTML(tsFrom, tsTo *TraceStats) {
	var data [][]string

	if tsTo == nil {
		for _, s := range tsFrom.ScenarioStats {
			data = append(data, p.GenerateTraceLine(s, true))
		}
	} else {
		for _, to := range tsTo.ScenarioStats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = append(data, p.GenerateTraceLine(to, false))
			} else {
				data = append(data, p.GenerateTraceLineWithDiff(from, to, false))
			}
		}
	}
	content, _ := html.RenderTableWithGridJS("alp", p.headers, data, p.printOptions.paginationLimit)
	fmt.Println(content)
}

func anyToFloat64(v interface{}) float64 {
	var f float64
	switch v.(type) {
	case int:
		f = float64(v.(int))
	case uint:
		f = float64(v.(uint))
	case uint64:
		f = float64(v.(uint64))
	case *uint64:
		f = float64(*v.(*uint64))
	case float64:
		f = float64(v.(float64))
	case *float64:
		f = float64(*v.(*float64))
	default:
		fmt.Fprintf(os.Stderr, "unknown type: %T", v)
	}
	return f
}
