package plist

import (
	"encoding/hex"
	_ "errors"
	"fmt"
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
			fmt.Fprintf(p.writer, `%s=`, strconv.QuoteToASCII(k))
			p.writePlistValue(dict.values[i])
			p.writer.Write([]byte(`;`))
		}
		p.writer.Write([]byte(`}`))
	case Array:
		p.writer.Write([]byte(`(`))
		values := pval.value.([]*plistValue)
		for i, v := range values {
			p.writePlistValue(v)
			if i < len(values)-1 {
				fmt.Fprintf(p.writer, ",")
			}
		}
		p.writer.Write([]byte(`)`))
	case String:
		fmt.Fprintf(p.writer, `%s`, strconv.QuoteToASCII(pval.value.(string)))
	case Integer:
		//fmt.Fprintf(p.writer, `"%v"`, pval.value)
		fmt.Fprintf(p.writer, `<*I%v>`, pval.value)
	case Real:
		//fmt.Fprintf(p.writer, `"%v"`, pval.value.(sizedFloat).value)
		fmt.Fprintf(p.writer, `<*R%v>`, pval.value.(sizedFloat).value)
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
		fmt.Fprintf(p.writer, `<%s>`, string(hexencoded))
	case Date:
		fmt.Fprintf(p.writer, `<*D%v>`, pval.value.(time.Time).In(time.UTC).Format(textPlistTimeLayout))
	}
}
