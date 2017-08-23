package genx

import "regexp"

var pkgWithTypeRE = regexp.MustCompile(`^(\w+):(?:(.*)/([\w\d_-]+)(?:#([\w\d]+))?\.)?([*\w\d]+)$`)

func parsePackageWithType(v string) (typ, name, pkg, sel string) {
	p := pkgWithTypeRE.FindAllStringSubmatch(v, -1)
	if len(p) != 1 {
		return
	}

	parts := p[0]
	typ, pkg = parts[1], parts[2]+"/"+parts[3]
	if name = parts[4]; name != "" {
		parts[3] = name
	}
	if parts[5][0] == '*' {
		sel = "*" + parts[3] + "." + parts[5][1:]
	} else {
		sel = parts[3] + "." + parts[5]
	}
	return
}
