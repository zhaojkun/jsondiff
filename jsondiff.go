package jsondiff

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
)

type Difference int

const (
	FullMatch Difference = iota
	SupersetMatch
	NoMatch
	FirstArgIsInvalidJson
	SecondArgIsInvalidJson
	BothArgsAreInvalidJson
)

func (d Difference) String() string {
	switch d {
	case FullMatch:
		return "FullMatch"
	case SupersetMatch:
		return "SupersetMatch"
	case NoMatch:
		return "NoMatch"
	case FirstArgIsInvalidJson:
		return "FirstArgIsInvalidJson"
	case SecondArgIsInvalidJson:
		return "SecondArgIsInvalidJson"
	case BothArgsAreInvalidJson:
		return "BothArgsAreInvalidJson"
	}
	return "Invalid"
}

type Tag struct {
	Begin string
	End   string
}

type Options struct {
	Normal            Tag
	Added             Tag
	Removed           Tag
	Changed           Tag
	Prefix            string
	Indent            string
	PrintTypes        bool
	FuzzyFields       []string
	IgnoreFields      []string
	StringAsMapFields []string
	NullAsEmpty       bool
}

// Provides a set of options that are well suited for console output. Options
// use ANSI foreground color escape sequences to highlight changes.
func DefaultConsoleOptions() Options {
	return Options{
		Added:   Tag{Begin: "\033[0;32m", End: "\033[0m"},
		Removed: Tag{Begin: "\033[0;31m", End: "\033[0m"},
		Changed: Tag{Begin: "\033[0;33m", End: "\033[0m"},
		Indent:  "    ",
	}
}

// Provides a set of options that are well suited for HTML output. Works best
// inside <pre> tag.
func DefaultHTMLOptions() Options {
	return Options{
		Added:   Tag{Begin: `<span style="background-color: #8bff7f">`, End: `</span>`},
		Removed: Tag{Begin: `<span style="background-color: #fd7f7f">`, End: `</span>`},
		Changed: Tag{Begin: `<span style="background-color: #fcff7f">`, End: `</span>`},
		Indent:  "    ",
	}
}

type context struct {
	opts              *Options
	level             int
	lastTag           *Tag
	diff              Difference
	curKey            string
	fuzzyFields       map[string]struct{}
	ignoreFields      map[string]struct{}
	stringAsMapFields map[string]struct{}
}

func (ctx *context) newline(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
	if ctx.lastTag != nil {
		buf.WriteString(ctx.lastTag.End)
	}
	buf.WriteString("\n")
	buf.WriteString(ctx.opts.Prefix)
	for i := 0; i < ctx.level; i++ {
		buf.WriteString(ctx.opts.Indent)
	}
	if ctx.lastTag != nil {
		buf.WriteString(ctx.lastTag.Begin)
	}
}

func (ctx *context) key(buf *bytes.Buffer, k string) {
	ctx.curKey = k
	buf.WriteString(strconv.Quote(k))
	buf.WriteString(": ")
}

func (ctx *context) writeValue(buf *bytes.Buffer, v interface{}, full bool) {
	switch vv := v.(type) {
	case bool:
		buf.WriteString(strconv.FormatBool(vv))
	case json.Number:
		buf.WriteString(string(vv))
	case string:
		buf.WriteString(strconv.Quote(vv))
	case []interface{}:
		if full {
			if len(vv) == 0 {
				buf.WriteString("[")
			} else {
				ctx.level++
				ctx.newline(buf, "[")
			}
			for i, v := range vv {
				ctx.writeValue(buf, v, true)
				if i != len(vv)-1 {
					ctx.newline(buf, ",")
				} else {
					ctx.level--
					ctx.newline(buf, "")
				}
			}
			buf.WriteString("]")
		} else {
			buf.WriteString("[]")
		}
	case map[string]interface{}:
		if full {
			if len(vv) == 0 {
				buf.WriteString("{")
			} else {
				ctx.level++
				ctx.newline(buf, "{")
			}
			i := 0
			for k, v := range vv {
				ctx.key(buf, k)
				ctx.writeValue(buf, v, true)
				if i != len(vv)-1 {
					ctx.newline(buf, ",")
				} else {
					ctx.level--
					ctx.newline(buf, "")
				}
				i++
			}
			buf.WriteString("}")
		} else {
			buf.WriteString("{}")
		}
	default:
		buf.WriteString("null")
	}

	ctx.writeTypeMaybe(buf, v)
}

func (ctx *context) writeTypeMaybe(buf *bytes.Buffer, v interface{}) {
	if ctx.opts.PrintTypes {
		buf.WriteString(" ")
		ctx.writeType(buf, v)
	}
}

func (ctx *context) writeType(buf *bytes.Buffer, v interface{}) {
	switch v.(type) {
	case bool:
		buf.WriteString("(boolean)")
	case json.Number:
		buf.WriteString("(number)")
	case string:
		buf.WriteString("(string)")
	case []interface{}:
		buf.WriteString("(array)")
	case map[string]interface{}:
		buf.WriteString("(object)")
	default:
		buf.WriteString("(null)")
	}
}

func (ctx *context) writeMismatch(buf *bytes.Buffer, a, b interface{}) {
	ctx.writeValue(buf, a, false)
	buf.WriteString(" => ")
	ctx.writeValue(buf, b, false)
}

func (ctx *context) tag(buf *bytes.Buffer, tag *Tag) {
	if ctx.lastTag == tag {
		return
	} else if ctx.lastTag != nil {
		buf.WriteString(ctx.lastTag.End)
	}
	buf.WriteString(tag.Begin)
	ctx.lastTag = tag
}

func (ctx *context) result(d Difference) {
	if d == NoMatch {
		ctx.diff = NoMatch
	} else if d == SupersetMatch && ctx.diff != NoMatch {
		ctx.diff = SupersetMatch
	} else if ctx.diff != NoMatch && ctx.diff != SupersetMatch {
		ctx.diff = FullMatch
	}
}

func (ctx *context) printMismatch(buf *bytes.Buffer, a, b interface{}) {
	ctx.tag(buf, &ctx.opts.Changed)
	ctx.writeMismatch(buf, a, b)
}

func (ctx *context) printStringDiff(buf *bytes.Buffer, aa string, b interface{}) Difference {
	failedFn := func() Difference {
		ctx.printMismatch(buf, aa, b)
		ctx.result(NoMatch)
		return NoMatch
	}
	bb, ok := b.(string)
	if !ok {
		return failedFn()
	}
	if aa == bb {
		return FullMatch
	}
	_, isStringAsMap := ctx.stringAsMapFields[ctx.curKey]
	if !isStringAsMap {
		return failedFn()
	}
	diff, msg := Compare([]byte(aa), []byte(bb), ctx.opts)
	if diff != FullMatch {
		buf.WriteString(msg)
		ctx.result(diff)
		return diff
	}
	return FullMatch
}
func (ctx *context) isStringDiff(aa string, b interface{}) bool {
	bb, ok := b.(string)
	if !ok {
		return true
	}
	if aa == bb {
		return false
	}
	_, isStringAsMap := ctx.stringAsMapFields[ctx.curKey]
	if !isStringAsMap {
		return true
	}
	diff, _ := Compare([]byte(aa), []byte(bb), &Options{})
	return diff != FullMatch
}

func (ctx *context) isZeroLen(a, b interface{}) bool {
	data := a
	if data == nil {
		data = b
	}
	sd, ok := data.([]interface{})
	if ok && len(sd) == 0 {
		return true
	}
	sm, ok := data.(map[string]interface{})
	if ok && len(sm) == 0 {
		return true
	}
	return false
}

func (ctx *context) printDiff(buf *bytes.Buffer, a, b interface{}) Difference {
	_, isFuzzy := ctx.fuzzyFields[ctx.curKey]
	if a == nil || b == nil {
		if isFuzzy || (a == nil && b == nil) || (ctx.opts.NullAsEmpty && ctx.isZeroLen(a, b)) {
			ctx.tag(buf, &ctx.opts.Normal)
			ctx.writeValue(buf, a, false)
			ctx.result(FullMatch)
			return FullMatch
		} else {
			ctx.printMismatch(buf, a, b)
			ctx.result(NoMatch)
			return NoMatch
		}
	}

	ka := reflect.TypeOf(a).Kind()
	kb := reflect.TypeOf(b).Kind()
	if ka != kb {
		ctx.printMismatch(buf, a, b)
		ctx.result(NoMatch)
		return NoMatch
	}
	if isFuzzy {
		ctx.tag(buf, &ctx.opts.Normal)
		ctx.writeValue(buf, a, false)
		ctx.result(FullMatch)
		return FullMatch
	}
	switch ka {
	case reflect.Bool:
		if a.(bool) != b.(bool) {
			ctx.printMismatch(buf, a, b)
			ctx.result(NoMatch)
			return NoMatch
		}
	case reflect.String:
		switch aa := a.(type) {
		case json.Number:
			bb, ok := b.(json.Number)
			if !ok || aa != bb {
				ctx.printMismatch(buf, a, b)
				ctx.result(NoMatch)
				return NoMatch
			}
		case string:
			if diff := ctx.printStringDiff(buf, aa, b); diff != FullMatch {
				return diff
			}
		}
	case reflect.Slice:
		sa, sb := a.([]interface{}), b.([]interface{})
		salen, sblen := len(sa), len(sb)
		max := salen
		if sblen > max {
			max = sblen
		}
		ctx.tag(buf, &ctx.opts.Normal)
		if max == 0 {
			buf.WriteString("[")
		} else {
			ctx.level++
			ctx.newline(buf, "[")
		}
		sDiff := FullMatch
		isFirstKey := true
		for i := 0; i < max; i++ {
			itemDiff := FullMatch
			itemBuf := &bytes.Buffer{}
			if i < salen && i < sblen {
				itemDiff = ctx.printDiff(itemBuf, sa[i], sb[i])
			} else if i < salen {
				ctx.tag(itemBuf, &ctx.opts.Removed)
				ctx.writeValue(itemBuf, sa[i], true)
				ctx.result(SupersetMatch)
				itemDiff = SupersetMatch
			} else if i < sblen {
				ctx.tag(itemBuf, &ctx.opts.Added)
				ctx.writeValue(itemBuf, sb[i], true)
				ctx.result(NoMatch)
				itemDiff = NoMatch
			}
			if itemDiff != FullMatch {
				if isFirstKey {
					isFirstKey = false
				} else {
					ctx.newline(buf, ",")
				}
				sDiff = itemDiff
				buf.WriteString(itemBuf.String())
				ctx.tag(buf, &ctx.opts.Normal)
			}
		}
		ctx.level--
		ctx.newline(buf, "")
		buf.WriteString("]")
		ctx.writeTypeMaybe(buf, a)
		return sDiff
	case reflect.Map:
		ma, mb := a.(map[string]interface{}), b.(map[string]interface{})
		keysMap := make(map[string]bool)
		for k := range ma {
			keysMap[k] = true
		}
		for k := range mb {
			keysMap[k] = true
		}
		keys := make([]string, 0, len(keysMap))
		for k := range keysMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ctx.tag(buf, &ctx.opts.Normal)
		if len(keys) == 0 {
			buf.WriteString("{")
		} else {
			ctx.level++
			ctx.newline(buf, "{")
		}
		mDiff := FullMatch
		isfirstKey := true
		for _, k := range keys {
			if _, found := ctx.ignoreFields[k]; found {
				continue
			}
			itemBuf := &bytes.Buffer{}
			itemDiff := FullMatch
			va, aok := ma[k]
			vb, bok := mb[k]
			if aok && bok {
				ctx.key(itemBuf, k)
				itemDiff = ctx.printDiff(itemBuf, va, vb)
			} else if aok {
				ctx.tag(itemBuf, &ctx.opts.Removed)
				ctx.key(itemBuf, k)
				ctx.writeValue(itemBuf, va, true)
				ctx.result(SupersetMatch)
				itemDiff = SupersetMatch
			} else if bok {
				ctx.tag(itemBuf, &ctx.opts.Added)
				ctx.key(itemBuf, k)
				ctx.writeValue(itemBuf, vb, true)
				ctx.result(NoMatch)
				itemDiff = NoMatch
			}
			if itemDiff != FullMatch {
				if isfirstKey {
					isfirstKey = false
				} else {
					ctx.newline(buf, ",")
				}
				mDiff = itemDiff
				buf.WriteString(itemBuf.String())
				ctx.tag(buf, &ctx.opts.Normal)
			}
		}
		ctx.level--
		ctx.newline(buf, "")
		buf.WriteString("}")
		ctx.writeTypeMaybe(buf, a)
		return mDiff
	}
	ctx.tag(buf, &ctx.opts.Normal)
	ctx.writeValue(buf, a, true)
	ctx.result(FullMatch)
	return FullMatch
}

// Compares two JSON documents using given options. Returns difference type and
// a string describing differences.
//
// FullMatch means provided arguments are deeply equal.
//
// SupersetMatch means first argument is a superset of a second argument. In
// this context being a superset means that for each object or array in the
// hierarchy which don't match exactly, it must be a superset of another one.
// For example:
//
//     {"a": 123, "b": 456, "c": [7, 8, 9]}
//
// Is a superset of:
//
//     {"a": 123, "c": [7, 8]}
//
// NoMatch means there is no match.
//
// The rest of the difference types mean that one of or both JSON documents are
// invalid JSON.
//
// Returned string uses a format similar to pretty printed JSON to show the
// human-readable difference between provided JSON documents. It is important
// to understand that returned format is not a valid JSON and is not meant
// to be machine readable.
func Compare(a, b []byte, opts *Options) (Difference, string) {
	var av, bv interface{}
	da := json.NewDecoder(bytes.NewReader(a))
	da.UseNumber()
	db := json.NewDecoder(bytes.NewReader(b))
	db.UseNumber()
	errA := da.Decode(&av)
	errB := db.Decode(&bv)
	if errA != nil && errB != nil {
		return BothArgsAreInvalidJson, "both arguments are invalid json"
	}
	if errA != nil {
		return FirstArgIsInvalidJson, "first argument is invalid json"
	}
	if errB != nil {
		return SecondArgIsInvalidJson, "second argument is invalid json"
	}

	ctx := context{opts: opts}
	ctx.fuzzyFields = sliceToSet(opts.FuzzyFields)
	ctx.ignoreFields = sliceToSet(opts.IgnoreFields)
	ctx.stringAsMapFields = sliceToSet(opts.StringAsMapFields)
	var buf bytes.Buffer
	ctx.printDiff(&buf, av, bv)
	if ctx.diff == FullMatch {
		return FullMatch, ""
	}
	if ctx.lastTag != nil {
		buf.WriteString(ctx.lastTag.End)
	}
	return ctx.diff, buf.String()
}

func sliceToSet(src []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, k := range src {
		m[k] = struct{}{}
	}
	return m
}
