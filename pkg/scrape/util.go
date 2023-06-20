package scrape

import (
	"github.com/mozillazg/go-pinyin"
)

func ParseCnToEn(cn string) string {
	a := pinyin.NewArgs()
	a.Separator = ""
	a.Fallback = func(r rune, a pinyin.Args) []string {
		if string(r) == " " {
			return []string{}
		}

		return []string{string(r)}
	}

	return pinyin.Slug(cn, a)
}
