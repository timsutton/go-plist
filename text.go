package plist

import (
	"bufio"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"time"
)
import "fmt"

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
		if pval.value.(signedInt).signed {
			io.WriteString(p.writer, strconv.FormatInt(int64(pval.value.(signedInt).value), 10))
		} else {
			io.WriteString(p.writer, strconv.FormatUint(pval.value.(signedInt).value, 10))
		}
	case Real:
		io.WriteString(p.writer, strconv.FormatFloat(pval.value.(sizedFloat).value, 'g', -1, 64))
	case Boolean:
		b := pval.value.(bool)
		if b {
			p.writer.Write([]byte(`1`))
		} else {
			p.writer.Write([]byte(`0`))
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

type readPeeker interface {
	io.Reader
	io.ByteScanner
	Peek(n int) ([]byte, error)
}

type textPlistParser struct {
	reader readPeeker
}

func (p *textPlistParser) parseDocument() *plistValue {
	return p.parsePlistValue()
}

func (p *textPlistParser) parseQuotedString() *plistValue {
	s := ""
	for {
		byt, err := p.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		c := rune(byt)
		fmt.Println("read", c)
		if c == '"' {
			break
		} else if c == '\\' {
			byt, err = p.reader.ReadByte()
			if err != nil {
				panic(err)
			}
			switch byt {
			case 'a':
				c = '\a'
			case 'b':
				c = '\b'
			case 'v':
				c = '\v'
			case 'f':
				c = '\f'
			case '\\':
				c = '\\'
			case '"':
				c = '"'
			case 't':
				c = '\t'
			case 'r':
				c = '\r'
			case 'n':
				c = '\n'
			case 'x':
				hex := make([]byte, 2)
				p.reader.Read(hex)
				newc, err := strconv.ParseInt(string(hex), 16, 16)
				if err != nil && err != io.EOF {
					panic(err)
				}
				c = rune(newc)
			case 'u', 'U':
				hex := make([]byte, 4)
				p.reader.Read(hex)
				newc, err := strconv.ParseInt(string(hex), 16, 16)
				if err != nil && err != io.EOF {
					panic(err)
				}
				c = rune(newc)
			}
		}
		s += string(c)
	}
	fmt.Println("GOT QUOTED STRING", s)
	return &plistValue{String, s}
}

func (p *textPlistParser) parseUnquotedString() *plistValue {
	s := ""
	for {
		c, err := p.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		if quotable[c/64]&(1<<(c%64)) > 0 {
			p.reader.UnreadByte()
			break
		}
		s += string(c)
	}
	fmt.Println("GOT UNQUOTED STRING", s)
	return &plistValue{String, s}
}

func (p *textPlistParser) parseDictionary() *plistValue {
	subval := make(map[string]*plistValue)
	for {
		buf, err := p.reader.Peek(1)
		if err != nil {
			panic(err)
		}
		c := buf[0]

		var keypv *plistValue
		if c == '"' {
			p.reader.ReadByte() // read and discard opening quote
			keypv = p.parseQuotedString()
		} else {
			keypv = p.parseUnquotedString()
		}
		if keypv == nil || keypv.value.(string) == "" {
			// TODO better error
			panic(errors.New("plist: missing dictionary key"))
		}

		c, err = p.reader.ReadByte()
		if err != nil {
			panic(err)
		}

		if c != '=' {
			panic(errors.New("plist: missing = in dictionary"))
		}

		val := p.parsePlistValue()

		c, err = p.reader.ReadByte()
		if err != nil {
			panic(err)
		}

		if c != ';' {
			panic(errors.New("plist: missing ; in dictionary"))
		}

		subval[keypv.value.(string)] = val

		buf, err = p.reader.Peek(1)
		if err != nil {
			panic(err)
		}
		c = buf[0]
		if c == '}' {
			p.reader.ReadByte()
			break
		}

	}
	return &plistValue{Dictionary, &dictionary{m: subval}}
}

func (p *textPlistParser) parseArray() *plistValue {
	subval := make([]*plistValue, 0, 10)
	for {
		buf, err := p.reader.Peek(1)
		fmt.Println(buf)
		if err != nil {
			panic(err)
		}
		c := buf[0]

		if c == ',' {
			p.reader.ReadByte()
			continue
		}

		if c == ')' {
			p.reader.ReadByte()
			break
		}

		subval = append(subval, p.parsePlistValue())
	}
	return &plistValue{Array, subval}
}

func (p *textPlistParser) parsePlistValue() *plistValue {
	for {
		c, err := p.reader.ReadByte()
		if err != nil && err != io.EOF {
			panic(err)
		}
		switch {
		// chug whitespace
		case whitespace[c/64]&(1<<(c%64)) > 0:
			continue
		case c == '<':
			for {
				x, _ := p.reader.ReadByte()
				if x == '>' {
					break
				}
			}
			return &plistValue{Data, []byte{}}
			//return p.parseData()
		case c == '"':
			return p.parseQuotedString()
		case c == '{':
			return p.parseDictionary()
		case c == '(':
			return p.parseArray()
		default:
			p.reader.UnreadByte() // Place back in buffer for parseUnquotedString
			return p.parseUnquotedString()
		}
	}
	return nil
}

func newTextPlistParser(r io.Reader) *textPlistParser {
	var reader readPeeker
	if rd, ok := r.(readPeeker); ok {
		reader = rd
	} else {
		reader = bufio.NewReader(r)
	}
	return &textPlistParser{reader: reader}
}
