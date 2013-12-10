package plist

import (
	"encoding/hex"
	_ "errors"
	"io"
	_ "math"
	"strconv"
	"time"
)

type textPlistGenerator struct {
	writer io.Writer
}

var (
	textPlistTimeLayout = "2006-01-02 15:04:05 -0700"
)

func (p *textPlistGenerator) generateDocument(pval *plistValue) {
	p.writePlistValue(pval)
}

func plistQuotedString(str string) string {
	s := ""
	quot := false
	for _, r := range str {
		if r > 0xFF {
			quot = true
			s += `\U`
			s += strconv.FormatInt(int64(r), 16)
		} else if r > 0x7F {
			quot = true
			s += `\`
			s += strconv.FormatInt(int64(r), 8)
		} else {
			if quoteRequired(uint8(r)) {
				quot = true
			}

			switch uint8(r) {
			case '\a':
				s += `\a`
			case '\b':
				s += `\b`
			case '\v':
				s += `\v`
			case '\f':
				s += `\f`
			case '\\':
				s += `\\`
			case '"':
				s += `\"`
			case '\t', '\r', '\n':
				fallthrough
			default:
				s += string(r)
			}
		}
	}
	if quot {
		s = `"` + s + `"`
	}
	return s
}

func (p *textPlistGenerator) writePlistValue(pval *plistValue) {
	if pval == nil {
		return
	}

	switch pval.kind {
	case Dictionary:
		p.writer.Write([]byte(`{`))
		dict := pval.value.(*dictionary)
		dict.populateArrays()
		for i, k := range dict.keys {
			io.WriteString(p.writer, plistQuotedString(k)+`=`)
			p.writePlistValue(dict.values[i])
			p.writer.Write([]byte(`;`))
		}
		p.writer.Write([]byte(`}`))
	case Array:
		p.writer.Write([]byte(`(`))
		values := pval.value.([]*plistValue)
		for _, v := range values {
			p.writePlistValue(v)
			p.writer.Write([]byte(`,`))
		}
		p.writer.Write([]byte(`)`))
	case String:
		io.WriteString(p.writer, plistQuotedString(pval.value.(string)))
	case Integer:
		io.WriteString(p.writer, strconv.FormatUint(pval.value.(uint64), 10))
	case Real:
		io.WriteString(p.writer, strconv.FormatFloat(pval.value.(sizedFloat).value, 'g', -1, 64))
	case Boolean:
		b := pval.value.(bool)
		if b {
			p.writer.Write([]byte(`<*BY>`))
		} else {
			p.writer.Write([]byte(`<*BN>`))
		}
	case Data:
		b := pval.value.([]byte)
		hexencoded := make([]byte, hex.EncodedLen(len(b)))
		hex.Encode(hexencoded, b)
		io.WriteString(p.writer, `<`+string(hexencoded)+`>`)
	case Date:
		io.WriteString(p.writer, pval.value.(time.Time).In(time.UTC).Format(textPlistTimeLayout))
	}
}
