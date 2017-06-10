package csscut

import (
	"bytes"
	"crypto/sha512"
	"encoding/gob"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	levelDb *leveldb.DB
)

// Очищаем базу
func levelDbClean(path string) (err error) {
	err = os.RemoveAll(path)
	if err != nil {
		log.Println("[error]", err)
	}
	return
}

// Открываем хранилище
func openLevelDb(path string) (err error) {
	levelDb, err = leveldb.OpenFile(path, &opt.Options{
		NoSync: true,
	})
	if err != nil {
		log.Println("[error]", err)
		return
	}

	return
}

// Получаем ключ html
func getLevelDbKey(html string) (b []byte) {
	// Сначала получаем тэги
	tagsmap := make(map[string]bool)
	for _, st := range htmlTagReg.FindAllStringSubmatch(html, -1) {
		tagsmap[st[1]] = true
	}

	// Формируем срез и сортируем его
	data := make([]string, len(tagsmap))
	var i int
	for k := range tagsmap {
		data[i] = k
		i++
	}
	sort.Strings(data)

	// Добавляем классы в срез тэгов
	for _, st := range htmlClassReg.FindAllStringSubmatch(html, -1) {
		data = append(data, st[1])
	}

	h := sha512.New()
	h.Write([]byte(strings.Join(data, ";")))
	b = h.Sum(nil)
	return
}

// Получаем значение из кэша
func getCache(key []byte) (o styleData, err error) {
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	// Ищем стили в кэше
	css, err := levelDb.Get(key, nil)
	if err != nil && err.Error() != "leveldb: not found" {
		log.Println("[error]", err)
		return
	}

	// Если нашли слили
	if err == nil {
		o, err = fromGob(css)
		if err != nil {
			log.Println("[error]", err)
			return
		}
	}

	return
}

// Демон стилей
func levelDbDaemon() {
	for d := range cacheChan {
		levelDbAddCache(d)
	}
}

// Добавляем стиль в кэш
func levelDbAddCache(d cssData) {
	// Проверяем наличие в кэше
	_, err := getCache(d.Key)
	if err != nil && err.Error() != "leveldb: not found" {
		log.Println("[error]", err)
		return
	}
	// Если такой стиль уже в кэше
	if err == nil || len(d.Styles) == 0 {
		return
	}

	// Создаем json для uncss
	js, err := json.Marshal(uncssData{Paths: d.Styles, Html: d.Html})
	if err != nil {
		log.Println("[error]", err)
		return
	}

	// Пишем временный файл
	var f *os.File
	if f, err = ioutil.TempFile("/tmp/", "uncss_tmpl_"); err != nil {
		log.Println("[error]", err)
		return
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(js); err != nil {
		log.Println("[error]", err)
		return
	}
	if err = f.Close(); err != nil {
		log.Println("[error]", err)
		return
	}

	// Убираем лишние стили
	cmd := exec.Command(initObj.NodeScript, initObj.UncssScript, f.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("[error]", string(out))
		log.Println("[error]", err)
		return
	}

	// Создаем объект с данныеми о стиле
	o := styleData{
		Mtime: make(map[string]int),
		Css:   uncssCommentReg.ReplaceAll(out, []byte("")),
	}

	// Превращаем объект в gob
	b, err := toGob(o)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	cacheLock.RLock()
	defer cacheLock.RUnlock()
	// Пишем инфу в базу
	err = levelDb.Put(d.Key, b, nil)
	if err != nil {
		log.Println("[error]", err)
		return
	}
}

// Конвертация из gob
func fromGob(b []byte) (o styleData, err error) {
	var buf bytes.Buffer
	_, err = buf.Write(b)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&o)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	return
}

// Конвертация в gob
func toGob(o styleData) (b []byte, err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(o)
	if err != nil {
		log.Println("[error]", err)
		return
	}

	b = buf.Bytes()
	return
}
