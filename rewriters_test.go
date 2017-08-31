package genx_test

import (
	"io/ioutil"
	"log"
	"regexp"
	"testing"

	"github.com/OneOfOne/genx"
)

func init() {
	log.SetFlags(log.Lshortfile)
}
func TestAllTypes(t *testing.T) {
	testCases := []struct {
		Name        string
		Input       map[string]string
		FailIfMatch *regexp.Regexp
	}{
		{
			"Delete:Type:interface{}",
			map[string]string{
				"type:interface{}": "-",
			},
			regexp.MustCompile(`interface\{\}`),
		},
		{
			"Delete:Type:TypeWithKT",
			map[string]string{"type:TypeWithKT": "-"},
			regexp.MustCompile(`TypeWithKT`),
		},
		{
			"Delete:Field:RemoveMe",
			map[string]string{"field:RemoveMe": "-"},
			regexp.MustCompile(`RemoveMe`),
		},
		{
			"Rename:Field:Call",
			map[string]string{"field:Call": "NewFuncName"},
			regexp.MustCompile(`\.Call`),
		},
		{
			"Type:interface{}=int",
			map[string]string{
				"type:VT":          "int",
				"type:interface{}": "int",
			},
			regexp.MustCompile(`interface\{\}`),
		},
		{
			"Type:KT=int,VT=int",
			map[string]string{
				"type:KT": "uint64",
				"type:VT": "int",
			},
			regexp.MustCompile(`KT|VT`),
		},
		{
			"Func:DoStuff",
			map[string]string{
				"func:DoStuff": "-",
			},
			// regexp.MustCompile(`DoStuff\(|\sTwo\(`), // TODO: fix #5
			regexp.MustCompile(`DoStuff\(`),
		},
	}
	src, err := ioutil.ReadFile("./all_types.go")
	fatalIf(t, err)
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			g := genx.New("", tc.Input)
			pf, err := g.Parse("src.go", src)
			if err != nil {
				t.Errorf("%+v\n%s", err, pf.Src)
				return
			}
			if tc.FailIfMatch.Match(pf.Src) {
				t.Errorf("%s matched :(\n%s", tc.FailIfMatch, pf.Src)
			}
		})
	}
}

func fatalIf(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
