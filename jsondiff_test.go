package jsondiff

import (
	"io/ioutil"
	"log"
	"testing"
)

var cases = []struct {
	a      string
	b      string
	result Difference
}{
	{`{"a": 5}`, `["a"]`, NoMatch},
	{`{"a": 5}`, `{"a": 6}`, NoMatch},
	{`{"a": 5}`, `{"a": true}`, NoMatch},
	{`{"a": 5}`, `{"a": 5}`, FullMatch},
	{`{"a": 5}`, `{"a": 5, "b": 6}`, NoMatch},
	{`{"a": 5, "b": 6}`, `{"a": 5}`, SupersetMatch},
	{`{"a": 5, "b": 6}`, `{"b": 6}`, SupersetMatch},
	{`{"a": null}`, `{"a": 1}`, NoMatch},
	{`{"a": null}`, `{"a": null}`, FullMatch},
	{`{"a": "null"}`, `{"a": null}`, NoMatch},
	{`{"a": 3.1415}`, `{"a": 3.14156}`, NoMatch},
	{`{"a": 3.1415}`, `{"a": 3.1415}`, FullMatch},
	{`{"a": 4213123123}`, `{"a": "4213123123"}`, NoMatch},
	{`{"a": 4213123123}`, `{"a": 4213123123}`, FullMatch},
	{`{"a": 4213123123,"fuzz1":1,"fuzz2":13,"inner":{"e":"f"}}`, `{"a": 4213123123,"fuzz2":2,"inner":{"e":"f"}}`, FullMatch},
	{`{"stringAsMap":"{\"a\":1,\"b\":2}"}`, `{"stringAsMap":"{\"b\":2,\"a\":1}"}`, FullMatch},
	{`{"stringAsMap":"{\"a\":1,\"b\":2,\"c\":3}"}`, `{"stringAsMap":"{\"b\":2,\"a\":1}"}`, SupersetMatch},
	{`{"stringAsMap":"{\"a\":1,\"b\":2,\"c\":3}"}`, `{"stringAsMap":"{\"b\":2,\"a\":1,\"c\":4}"}`, NoMatch},
	{`{}`, `null`, FullMatch},
	{`{"key":null}`, `{"key":{}}`, FullMatch},
	{`{"key":null}`, `{}`, SupersetMatch},
}

func TestCompare(t *testing.T) {
	opts := DefaultConsoleOptions()
	opts.IgnoreFields = []string{"fuzz1"}
	opts.FuzzyFields = []string{"fuzz2"}
	opts.StringAsMapFields = []string{"stringAsMap"}
	opts.PrintTypes = false
	opts.NullAsEmpty = true
	for i, c := range cases {
		result, msg := Compare([]byte(c.a), []byte(c.b), &opts)
		log.Println(msg)
		if result != c.result {
			t.Errorf("case %d failed, got: %s, expected: %s", i, result, c.result)
		}
	}
}

func TestCompareJson(t *testing.T) {
	buf1, _ := ioutil.ReadFile("data1.json")
	buf2, _ := ioutil.ReadFile("data2.json")
	opts := DefaultConsoleOptions()
	opts.FuzzyFields = []string{"UrlList", "UserCount"}
	opts.StringAsMapFields = []string{"LinkRawData", "LinkSendData", "log_extra"}
	_, msg := Compare(buf1, buf2, &opts)
	_ = msg
	//	fmt.Println(msg)
}
