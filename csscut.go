package csscut

import (
	"crypto/md5"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

var (
	cacheLock sync.RWMutex
	cacheChan chan cssData

	initObj InitObj
)

// Инициализация
func Init(o InitObj) (err error) {
	initObj = o
	cacheChan = make(chan cssData, 100)

	if initObj.CleanOnStart {
		err = levelDbClean(initObj.LevelDbPath)
		if err != nil {
			log.Fatalln("[fatal]", err)
			return
		}
	}

	// Открываем базу
	err = openLevelDb(initObj.LevelDbPath)
	if err != nil {
		log.Fatalln("[fatal]", err)
		return
	}

	go levelDbDaemon()

	return
}

// Удаляем лишний css и вставляем в html
func CutAndInject(html string) (nhtml string, err error) {
	cssstr, err := GetCutCss(html)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	nhtml = InjectStyle(html, cssstr)
	return
}

// Получаем порезанный css
func GetCutCss(html string) (cssstr string, err error) {
	// Получаем список стилей из html
	styles := getStyleFiles(html)

	// Данные по стилю
	cdata := cssData{
		Styles: styles,
		Html:   html,
		Key:    getLevelDbKey(html),
	}

	// Проверяем наличие в кэше
	co, err := getCache(cdata.Key)
	if err != nil && err.Error() != "leveldb: not found" {
		log.Println("[error]", err)
		return
	}

	// Если нашли стиль - его и возвращаем
	if err == nil {
		cssstr = string(co.Css)
		return
	}

	// Отправляем данные для формирования css
	cacheChan <- cdata

	// Возвращаем быстро обрезанный стиль
	return fastCut(html, styles)
}

// Вставляем стили в html
func InjectStyle(html, css string) (nhtml string) {
	nhtml = styleReg.ReplaceAllStringFunc(html, injectStyleRepl)
	nhtml = styleTargetMeta.ReplaceAllString(nhtml, "<style>"+css+"</style>")
	return
}

func injectStyleRepl(s string) string {
	if styleHrefReg.MatchString(s) {
		return ""
	}
	return s
}

// Быстро обрезаем стиль
func fastCut(html string, styles []string) (cssstr string, err error) {
	// Читаем стили и инлайним без сжатия
	cssstr, err = readStyles(styles)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	htmlmap := map[string]map[string]bool{
		"tagsmap":  make(map[string]bool),
		"classmap": make(map[string]bool),
		"idmap":    make(map[string]bool),
	}

	// Сначала получаем тэги
	for _, st := range htmlTagReg.FindAllStringSubmatch(html, -1) {
		htmlmap["tagsmap"][st[1]] = true
	}

	// Получаем стили из html
	for _, st := range htmlClassReg.FindAllStringSubmatch(html, -1) {
		for _, v := range strings.Split(st[1], " ") {
			htmlmap["classmap"][v] = true
		}
	}

	// Получаем id из html
	for _, st := range htmlIdReg.FindAllStringSubmatch(html, -1) {
		htmlmap["idmap"][st[1]] = true
	}

	// Убираем все медиа стили
	cssstrWithoutMedia := cssMediaReg.ReplaceAllString(cssstr, "")

	// Разбиваем css
	var key string
	var fulldata []string
	mcss := make(map[string]string)
	for _, v := range cssSeparateReg.Split(cssstrWithoutMedia, -1) {
		if key == "" {
			key = v
		} else {
			mcss[key] = v
			fulldata = append(fulldata, compileCssLine(key, v))
			key = ""
		}
	}

	// Парсим css
	data := parseCss(mcss, htmlmap)

	// Убираем повторы
	cssarr := map[[16]byte]string{}
	for _, v := range data {
		md5key := md5.Sum([]byte(v))
		_, ok := cssarr[md5key]
		if ok {
			continue
		}
		cssarr[md5key] = v
	}

	// Собираем конечный css
	for _, fd := range fulldata {
		md5key := md5.Sum([]byte(fd))
		_, ok := cssarr[md5key]
		if ok {
			continue
		}

		// Удаляем стили которых нету
		cssstr = strings.Replace(cssstr, fd, "", -1)
	}

	return
}

// Парсим css
func parseCss(mcss map[string]string, htmlmap map[string]map[string]bool) (data []string) {
	data = []string{}

	// Разбираем css
	for k, v := range mcss {
		// Смотрим html тэги
		for _, tag := range cssHtmlReg.FindAllStringSubmatch(k, -1) {
			for i, t := range tag {
				if i == 0 {
					continue
				}
				_, ok := htmlmap["tagsmap"][t]
				if ok || t == "*" {
					data = append(data, compileCssLine(k, v))
				}
			}
		}

		// Смотрим css классы
		for _, tag := range cssClassReg.FindAllStringSubmatch(k, -1) {
			for i, t := range tag {
				if i == 0 {
					continue
				}
				_, ok := htmlmap["classmap"][t]
				if ok {
					data = append(data, compileCssLine(k, v))
				}
			}
		}

		// Смотрим id
		for _, tag := range cssIdReg.FindAllStringSubmatch(k, -1) {
			for i, t := range tag {
				if i == 0 {
					continue
				}
				_, ok := htmlmap["idmap"][t]
				if ok {
					data = append(data, compileCssLine(k, v))
				}
			}
		}
	}

	return
}

// Собираем css строку
func compileCssLine(k, v string) string {
	return k + "{" + v + "}"
}

// Получаем список файлов стилей
func getStyleFiles(html string) (styles []string) {
	// Ищем все стили в html
	for _, st := range styleReg.FindAllStringSubmatch(html, -1) {
		for _, t := range st {
			href := styleHrefReg.FindStringSubmatch(t)
			if len(href) == 2 {
				styles = append(styles, initObj.WwwPath+href[1])
			}
		}
	}

	return
}

// Читаем стили
func readStyles(files []string) (css string, err error) {

	for _, path := range files {
		var b []byte
		b, err = ioutil.ReadFile(path)
		if err != nil {
			log.Println("[error]", err)
			return
		}

		css += string(b) + "\n"
	}

	return
}
