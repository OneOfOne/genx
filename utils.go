//go:generate git submodule foreach git pull origin master
//go:generate git submodule update --remote --checkout --recursive

package genx

import (
	"regexp"
	"sort"
)

func (g *GenX) OrderedRewriters() (out []string) {
	for k, v := range g.rewriters {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return
}

var pkgWithTypeRE = regexp.MustCompile(`^(?:(.*)/([\w\d_-]+)(?:#([\w\d]+))?\.)?([*\w\d]+)$`)

func parsePackageWithType(v string) (name, pkg, sel string) {
	p := pkgWithTypeRE.FindAllStringSubmatch(v, -1)
	if len(p) != 1 {
		return
	}

	parts := p[0]
	pkg = parts[1] + "/" + parts[2]
	if name = parts[3]; name != "" {
		parts[2] = name
	}
	if parts[4][0] == '*' {
		if parts[2] != "" {
			sel = "*" + parts[2] + "." + parts[4][1:]
		} else {
			sel = "*" + parts[4][1:]
		}
	} else if parts[2] != "" {
		sel = parts[2] + "." + parts[4]
	} else {
		sel = parts[4]
	}
	return
}

func regexpReplacer(src string, repl string) func(string) string {
	re := regexp.MustCompile(src)
	return func(in string) string {
		return re.ReplaceAllString(in, repl)
	}
}
