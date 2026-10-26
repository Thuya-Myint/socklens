package update

import (
	"fmt"
	"regexp"
	"strings"
)

type semver struct {
	core [3]string
	pre  []string
}

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func parseVersion(value string) (semver, error) {
	if len(value) > 256 {
		return semver{}, fmt.Errorf("version is too long")
	}
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return semver{}, fmt.Errorf("cannot compare version %q; install a tagged release", value)
	}
	result := semver{core: [3]string{match[1], match[2], match[3]}}
	if match[4] != "" {
		result.pre = strings.Split(match[4], ".")
		for _, part := range result.pre {
			if numeric(part) && len(part) > 1 && part[0] == '0' {
				return semver{}, fmt.Errorf("invalid prerelease version %q", value)
			}
		}
	}
	return result, nil
}
func numeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
func numberCompare(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func Compare(a, b string) (int, error) {
	left, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range left.core {
		if c := numberCompare(left.core[i], right.core[i]); c != 0 {
			return c, nil
		}
	}
	if len(left.pre) == 0 && len(right.pre) == 0 {
		return 0, nil
	}
	if len(left.pre) == 0 {
		return 1, nil
	}
	if len(right.pre) == 0 {
		return -1, nil
	}
	for i := 0; i < len(left.pre) && i < len(right.pre); i++ {
		a, b := left.pre[i], right.pre[i]
		if a == b {
			continue
		}
		an, bn := numeric(a), numeric(b)
		if an && bn {
			return numberCompare(a, b), nil
		}
		if an {
			return -1, nil
		}
		if bn {
			return 1, nil
		}
		return strings.Compare(a, b), nil
	}
	return numberCompare(fmt.Sprint(len(left.pre)), fmt.Sprint(len(right.pre))), nil
}
