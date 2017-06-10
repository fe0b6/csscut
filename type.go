package csscut

import (
	"regexp"
)

var (
	cssSeparateReg *regexp.Regexp
	cssHtmlReg     *regexp.Regexp
	cssClassReg    *regexp.Regexp
	cssIdReg       *regexp.Regexp
	cssMediaReg    *regexp.Regexp

	styleReg     *regexp.Regexp
	styleHrefReg *regexp.Regexp

	htmlTagReg   *regexp.Regexp
	htmlClassReg *regexp.Regexp
	htmlIdReg    *regexp.Regexp

	uncssCommentReg *regexp.Regexp

	styleTargetMeta *regexp.Regexp
)

func init() {
	cssSeparateReg = regexp.MustCompile("{|}")
	cssHtmlReg = regexp.MustCompile("(?:^\\s*|,\\s*)([a-zA-Z0-9\\*]+)")
	cssClassReg = regexp.MustCompile("\\.([a-zA-Z0-9_-]+)")
	cssIdReg = regexp.MustCompile("#([a-zA-Z0-9_-]+)")
	cssMediaReg = regexp.MustCompile("@media[^{]+{([^@]+(?:})\\s*})")

	htmlTagReg = regexp.MustCompile(`<([a-z0-9]+)[ />]`)
	htmlClassReg = regexp.MustCompile(`<[^>]+class="([^"]+)"(?:[^>]+>|>)`)
	htmlIdReg = regexp.MustCompile(`<[^>]+id="([^"]+)"(?:[^>]+>|>)`)

	styleReg = regexp.MustCompile(`<link[^>]+rel="stylesheet"[^>]+/>`)
	styleHrefReg = regexp.MustCompile(`href="(/[^/][^"]+)"`) // Берем только локальные стили

	styleTargetMeta = regexp.MustCompile(`<meta type="style"/>`)

	uncssCommentReg = regexp.MustCompile(`/\*\*\*[^\*]+\*\*\*/`)
}

type InitObj struct {
	WwwPath      string
	LevelDbPath  string
	NodeScript   string
	UncssScript  string
	CleanOnStart bool
}

// Структура для запроса inline css
type cssData struct {
	Key    []byte
	Html   string
	Styles []string
}

type styleData struct {
	Mtime map[string]int
	Css   []byte
}

type uncssData struct {
	Paths []string `json:"paths"`
	Html  string   `json:"html"`
}
