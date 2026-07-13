package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

type importXLSXSheet struct {
	Name string
	Rows [][]string
}

func parseEnterpriseMemberImportXLSX(data []byte) ([]EnterpriseMemberImportRow, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(reader.File))
	var totalUncompressed uint64
	for _, file := range reader.File {
		name := path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
		lower := strings.ToLower(name)
		if strings.Contains(lower, "vbaproject") || strings.Contains(lower, "externallinks/") || strings.Contains(lower, "embeddings/") || strings.Contains(lower, "oleobjects/") || strings.HasSuffix(lower, "connections.xml") {
			return nil, errors.New("xlsx contains unsupported active or external content")
		}
		totalUncompressed += file.UncompressedSize64
		if len(reader.File) > 100 || totalUncompressed > 50<<20 || (file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > 100) {
			return nil, errors.New("xlsx resource limits exceeded")
		}
		files[name] = file
	}
	workbookFile := files["xl/workbook.xml"]
	relsFile := files["xl/_rels/workbook.xml.rels"]
	if workbookFile == nil || relsFile == nil {
		return nil, errors.New("xlsx workbook metadata missing")
	}
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := decodeImportXLSXXML(workbookFile, &workbook); err != nil {
		return nil, err
	}
	if len(workbook.Sheets) == 0 || len(workbook.Sheets) > 3 {
		return nil, errors.New("xlsx must contain one to three sheets")
	}
	var rels struct {
		Relationships []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := decodeImportXLSXXML(relsFile, &rels); err != nil {
		return nil, err
	}
	relTargets := make(map[string]string)
	for _, rel := range rels.Relationships {
		target := strings.TrimPrefix(rel.Target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		relTargets[rel.ID] = path.Clean(target)
	}
	sharedStrings, err := parseImportXLSXSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheets := make(map[string]importXLSXSheet)
	for _, meta := range workbook.Sheets {
		file := files[relTargets[meta.RID]]
		if file == nil {
			return nil, fmt.Errorf("worksheet %s is missing", meta.Name)
		}
		rows, err := parseImportXLSXWorksheet(file, sharedStrings)
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %w", meta.Name, err)
		}
		sheets[strings.ToLower(strings.TrimSpace(meta.Name))] = importXLSXSheet{Name: meta.Name, Rows: rows}
	}
	return enterpriseMemberRowsFromXLSXSheets(sheets)
}

func decodeImportXLSXXML(file *zip.File, target any) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	return xml.NewDecoder(io.LimitReader(reader, 10<<20)).Decode(target)
}

func parseImportXLSXSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	decoder := xml.NewDecoder(io.LimitReader(reader, 20<<20))
	values := make([]string, 0)
	insideSI := false
	var current strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "si" {
				insideSI = true
				current.Reset()
			}
			if insideSI && value.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return nil, err
				}
				_, _ = current.WriteString(text)
			}
		case xml.EndElement:
			if value.Name.Local == "si" {
				if current.Len() > enterpriseMemberImportMaxCellBytes {
					return nil, errors.New("shared string too long")
				}
				values = append(values, current.String())
				insideSI = false
			}
		}
	}
	return values, nil
}

func parseImportXLSXWorksheet(file *zip.File, shared []string) ([][]string, error) {
	var sheet struct {
		Rows []struct {
			R     int `xml:"r,attr"`
			Cells []struct {
				Ref     string  `xml:"r,attr"`
				Type    string  `xml:"t,attr"`
				Value   string  `xml:"v"`
				Formula *string `xml:"f"`
				Inline  struct {
					Text string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := decodeImportXLSXXML(file, &sheet); err != nil {
		return nil, err
	}
	if len(sheet.Rows) > enterpriseMemberImportMaxRows+1 {
		return nil, errors.New("too many rows")
	}
	rows := make([][]string, 0, len(sheet.Rows))
	for _, sourceRow := range sheet.Rows {
		row := make([]string, 0)
		for _, cell := range sourceRow.Cells {
			if cell.Formula != nil {
				return nil, errors.New("formulas are not allowed")
			}
			column := importXLSXColumnIndex(cell.Ref)
			if column < 0 || column > 100 {
				return nil, errors.New("invalid or excessive columns")
			}
			for len(row) <= column {
				row = append(row, "")
			}
			value := cell.Value
			switch cell.Type {
			case "s":
				index, err := strconv.Atoi(cell.Value)
				if err != nil || index < 0 || index >= len(shared) {
					return nil, errors.New("invalid shared string index")
				}
				value = shared[index]
			case "inlineStr":
				value = cell.Inline.Text
			case "str":
				return nil, errors.New("formula string cells are not allowed")
			}
			if len(value) > enterpriseMemberImportMaxCellBytes {
				return nil, errors.New("cell value too long")
			}
			row[column] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func importXLSXColumnIndex(reference string) int {
	value := 0
	letters := 0
	for _, char := range reference {
		if char < 'A' || char > 'Z' {
			break
		}
		value = value*26 + int(char-'A'+1)
		letters++
	}
	if letters == 0 {
		return -1
	}
	return value - 1
}

func enterpriseMemberRowsFromXLSXSheets(sheets map[string]importXLSXSheet) ([]EnterpriseMemberImportRow, error) {
	membersSheet, ok := sheets["members"]
	if !ok || len(membersSheet.Rows) < 2 {
		return nil, errors.New("members sheet is required")
	}
	type memberData struct {
		code, name                                string
		limit, limit5h, limit1d, limit7d, opening float64
		groups                                    []int64
		errors                                    []string
		order                                     int
	}
	members := make(map[string]*memberData)
	headers := importHeaderIndex(membersSheet.Rows[0])
	for i, record := range membersSheet.Rows[1:] {
		if importRecordEmpty(record) {
			continue
		}
		code := importCell(record, headers, "member_code")
		limit, limitErr := parseImportAmount(importCell(record, headers, "monthly_limit_usd"))
		limit5h, limit5hErr := parseImportAmount(importCell(record, headers, "rate_limit_5h"))
		limit1d, limit1dErr := parseImportAmount(importCell(record, headers, "rate_limit_1d"))
		limit7d, limit7dErr := parseImportAmount(importCell(record, headers, "rate_limit_7d"))
		opening, openingErr := parseImportAmount(importCell(record, headers, "opening_used_usd"))
		item := &memberData{code: code, name: importCell(record, headers, "member_name"), limit: limit, limit5h: limit5h, limit1d: limit1d, limit7d: limit7d, opening: opening, order: i}
		if limitErr != nil {
			item.errors = append(item.errors, "invalid_monthly_limit")
		}
		if openingErr != nil {
			item.errors = append(item.errors, "invalid_opening_used")
		}
		if limit5hErr != nil {
			item.errors = append(item.errors, "invalid_rate_limit_5h")
		}
		if limit1dErr != nil {
			item.errors = append(item.errors, "invalid_rate_limit_1d")
		}
		if limit7dErr != nil {
			item.errors = append(item.errors, "invalid_rate_limit_7d")
		}
		members[strings.ToLower(strings.TrimSpace(code))] = item
	}
	if groupSheet, ok := sheets["membergroups"]; ok && len(groupSheet.Rows) > 1 {
		headers := importHeaderIndex(groupSheet.Rows[0])
		type orderedGroup struct {
			id    int64
			order int
		}
		groups := make(map[string][]orderedGroup)
		for _, record := range groupSheet.Rows[1:] {
			if importRecordEmpty(record) {
				continue
			}
			code := strings.ToLower(importCell(record, headers, "member_code"))
			id, idErr := strconv.ParseInt(importCell(record, headers, "group_id"), 10, 64)
			order, orderErr := strconv.Atoi(importCell(record, headers, "sort_order"))
			if member := members[code]; member == nil {
				continue
			} else if idErr != nil || id <= 0 || orderErr != nil || order < 0 {
				member.errors = append(member.errors, "invalid_member_group")
			} else {
				groups[code] = append(groups[code], orderedGroup{id, order})
			}
		}
		for code, values := range groups {
			sort.SliceStable(values, func(i, j int) bool { return values[i].order < values[j].order })
			for _, value := range values {
				members[code].groups = append(members[code].groups, value.id)
			}
		}
	}
	type keyData struct {
		name, key string
		quota     float64
		errors    []string
	}
	keys := make(map[string][]keyData)
	if keySheet, ok := sheets["keys"]; ok && len(keySheet.Rows) > 1 {
		headers := importHeaderIndex(keySheet.Rows[0])
		for _, record := range keySheet.Rows[1:] {
			if importRecordEmpty(record) {
				continue
			}
			code := strings.ToLower(importCell(record, headers, "member_code"))
			quota, quotaErr := parseImportAmount(importCell(record, headers, "key_quota_usd"))
			item := keyData{name: importCell(record, headers, "key_name"), key: importCell(record, headers, "api_key"), quota: quota}
			if quotaErr != nil {
				item.errors = append(item.errors, "invalid_key_quota")
			}
			if members[code] == nil {
				continue
			}
			keys[code] = append(keys[code], item)
		}
	}
	ordered := make([]*memberData, 0, len(members))
	for _, member := range members {
		ordered = append(ordered, member)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].order < ordered[j].order })
	rows := make([]EnterpriseMemberImportRow, 0)
	rowNumber := 2
	for _, member := range ordered {
		memberKeys := keys[strings.ToLower(strings.TrimSpace(member.code))]
		if len(memberKeys) == 0 {
			memberKeys = []keyData{{}}
		}
		for index, key := range memberKeys {
			opening := 0.0
			if index == 0 {
				opening = member.opening
			}
			errs := append([]string(nil), member.errors...)
			errs = append(errs, key.errors...)
			rows = append(rows, EnterpriseMemberImportRow{RowNumber: rowNumber, MemberCode: member.code, MemberName: member.name, MonthlyLimitUSD: member.limit, RateLimit5h: member.limit5h, RateLimit1d: member.limit1d, RateLimit7d: member.limit7d, OpeningUsedUSD: opening, KeyName: key.name, APIKeyCiphertext: key.key, KeyPresent: key.name != "" || key.key != "", KeyQuotaUSD: key.quota, GroupIDs: append([]int64(nil), member.groups...), Errors: errs, Warnings: []string{}})
			rowNumber++
		}
	}
	return rows, nil
}

func EnterpriseMemberImportXLSXTemplate() ([]byte, error) {
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet3.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Members" sheetId="1" r:id="rId1"/><sheet name="Keys" sheetId="2" r:id="rId2"/><sheet name="MemberGroups" sheetId="3" r:id="rId3"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet3.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   importXLSXTemplateSheet([][]string{{"成员编号", "成员名称", "5小时限额", "1天限额", "7天限额", "自然月预算（USD）", "初始已用额度（USD）"}, {"employee-001", "示例成员", "25", "50", "75", "100", "0"}}),
		"xl/worksheets/sheet2.xml":   importXLSXTemplateSheet([][]string{{"成员编号", "密钥名称", "API密钥", "密钥额度（USD）"}, {"employee-001", "主密钥", "", "0"}}),
		"xl/worksheets/sheet3.xml":   importXLSXTemplateSheet([][]string{{"成员编号", "分组ID", "顺序"}, {"employee-001", "1", "0"}}),
	}
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(writer, content); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func importXLSXTemplateSheet(rows [][]string) string {
	var builder strings.Builder
	_, _ = builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		_, _ = builder.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for colIndex, value := range row {
			ref := importXLSXColumnName(colIndex) + strconv.Itoa(rowIndex+1)
			_, _ = builder.WriteString(`<c r="` + ref + `" t="inlineStr"><is><t>`)
			_ = xml.EscapeText(&builder, []byte(value))
			_, _ = builder.WriteString(`</t></is></c>`)
		}
		_, _ = builder.WriteString(`</row>`)
	}
	_, _ = builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}
func importXLSXColumnName(index int) string {
	value := index + 1
	name := ""
	for value > 0 {
		value--
		name = string(rune('A'+value%26)) + name
		value /= 26
	}
	return name
}
