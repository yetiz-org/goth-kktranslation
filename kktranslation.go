package kktranslation

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/yetiz-org/goth-kklogger"
	"gopkg.in/yaml.v2"
)

var LangRootPath = "./resources/translation"
var TranslateFallback = true
var DefaultLang = "zh-tw"

type KKTranslation struct {
	langRootPath      func() string
	translateFallback func() bool
	defaultLang       func() string
	isDebug           func() bool

	langMap      sync.Map
	langFiles    []LangFile
	langLoadLock sync.Mutex
	fullLoaded   sync.Once

	emptyLangFile *LangFile
}

var defaultKKTranslation = newKKTranslation(
	func() string { return LangRootPath },
	func() bool { return TranslateFallback },
	func() string { return DefaultLang },
	_IsDebug,
)

func New() *KKTranslation {
	return NewWith(LangRootPath, TranslateFallback, DefaultLang)
}

func NewWith(langRootPath string, translateFallback bool, defaultLang string) *KKTranslation {
	if langRootPath == "" {
		langRootPath = LangRootPath
	}
	if defaultLang == "" {
		defaultLang = DefaultLang
	}

	lr := langRootPath
	tf := translateFallback
	dl := defaultLang
	return newKKTranslation(
		func() string { return lr },
		func() bool { return tf },
		func() string { return dl },
		_IsDebug,
	)
}

func NewWithProviders(
	langRootPath func() string,
	translateFallback func() bool,
	defaultLang func() string,
	isDebug func() bool,
) *KKTranslation {
	return newKKTranslation(langRootPath, translateFallback, defaultLang, isDebug)
}

func newKKTranslation(
	langRootPath func() string,
	translateFallback func() bool,
	defaultLang func() string,
	isDebug func() bool,
) *KKTranslation {
	k := &KKTranslation{
		langRootPath:      langRootPath,
		translateFallback: translateFallback,
		defaultLang:       defaultLang,
		isDebug:           isDebug,
	}
	k.emptyLangFile = &LangFile{t: k}
	return k
}

type LangFile struct {
	Version string            `yaml:"version"`
	Lang    string            `yaml:"lang"`
	Name    string            `yaml:"name"`
	Dict    map[string]string `yaml:"dict"`

	t *KKTranslation
}

func (k *KKTranslation) rootPath() string {
	if k == nil || k.langRootPath == nil {
		return LangRootPath
	}
	return k.langRootPath()
}

func (k *KKTranslation) fallbackEnabled() bool {
	if k == nil || k.translateFallback == nil {
		return TranslateFallback
	}
	return k.translateFallback()
}

func (k *KKTranslation) defaultLanguage() string {
	if k == nil || k.defaultLang == nil {
		return DefaultLang
	}
	return k.defaultLang()
}

func (k *KKTranslation) debugEnabled() bool {
	if k == nil || k.isDebug == nil {
		return _IsDebug()
	}
	return k.isDebug()
}

func (k *KKTranslation) empty() *LangFile {
	if k != nil && k.emptyLangFile != nil {
		return k.emptyLangFile
	}
	return &LangFile{}
}

func (k *KKTranslation) loadLangFile(lang string) *LangFile {
	lang = _LangNameNormalize(lang)
	if k.debugEnabled() {
		k.langMap.Delete(lang)
		if slang := strings.Split(lang, "-"); len(slang) > 1 {
			k.langMap.Delete(slang[0])
		}
	}

	if l, ok := k.langMap.Load(lang); ok {
		return l.(*LangFile)
	}

	k.langLoadLock.Lock()
	defer k.langLoadLock.Unlock()

	if l, ok := k.langMap.Load(lang); ok {
		return l.(*LangFile)
	}

	langFile := &LangFile{t: k}
	ml := ""
	if slang := strings.Split(lang, "-"); len(slang) > 1 {
		ml = slang[0]
	}

	if data, err := os.ReadFile(fmt.Sprintf("%s/%s.yaml", k.rootPath(), lang)); err == nil {
		if err := yaml.Unmarshal(data, langFile); err != nil {
			kklogger.WarnJ("KKTranslation.LoadLangFile", err.Error())
			return nil
		}

		langFile.t = k
		k.langMap.Store(lang, langFile)
		return langFile
	}

	if ml == "" {
		return nil
	}

	if data, err := os.ReadFile(fmt.Sprintf("%s/%s.yaml", k.rootPath(), ml)); err == nil {
		if err := yaml.Unmarshal(data, langFile); err != nil {
			kklogger.WarnJ("KKTranslation.LoadLangFile", err.Error())
			return nil
		}

		langFile.t = k
		k.langMap.Store(ml, langFile)
		k.langMap.Store(lang, langFile)
		return langFile
	}

	return nil
}

func (k *KKTranslation) loadAllLangFiles() []LangFile {
	files, e := os.ReadDir(k.rootPath())
	if e != nil {
		return []LangFile{}
	}

	out := make([]LangFile, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if langFile := k.loadLangFile(strings.Split(file.Name(), ".")[0]); langFile != nil {
			out = append(out, *langFile)
		}
	}

	return out
}

func (k *KKTranslation) LangFiles() []LangFile {
	if k.debugEnabled() {
		k.langMap.Range(func(key, _ any) bool {
			k.langMap.Delete(key)
			return true
		})
		return k.loadAllLangFiles()
	}

	k.fullLoaded.Do(func() {
		k.langFiles = k.loadAllLangFiles()
	})

	out := make([]LangFile, len(k.langFiles))
	copy(out, k.langFiles)
	return out
}

func (k *KKTranslation) GetLangFile(lang string) *LangFile {
	if langFile := k.loadLangFile(lang); langFile != nil {
		return langFile
	}
	if langFile := k.loadLangFile(k.defaultLanguage()); langFile != nil {
		return langFile
	}
	return k.empty()
}

func (k *KKTranslation) translate(l *LangFile, message string) string {
	if l == nil {
		return message
	}
	if m, f := l.Dict[message]; f {
		return m
	}

	ml := ""
	if slang := strings.Split(_LangNameNormalize(l.Lang), "-"); len(slang) > 1 {
		ml = slang[0]
	}
	if ml != "" {
		if lf := k.loadLangFile(ml); lf != nil {
			return k.translate(lf, message)
		}
	}

	lf := k.GetLangFile(k.defaultLanguage())
	if k.fallbackEnabled() && lf != nil && l.Lang != lf.Lang {
		return k.translate(lf, message)
	}

	return message
}

func LangFiles() []LangFile {
	return defaultKKTranslation.LangFiles()
}

func GetLangFile(lang string) *LangFile {
	return defaultKKTranslation.GetLangFile(lang)
}

func (l *LangFile) T(message string) string {
	if l == nil {
		return message
	}

	k := l.t
	if k == nil {
		k = defaultKKTranslation
	}
	return k.translate(l, message)
}

func _LangNameNormalize(lang string) string {
	return strings.ToLower(lang)
}

func _IsDebug() bool {
	v := os.Getenv("APP_DEBUG")
	if v == "" {
		v = os.Getenv("KKAPP_DEBUG")
	}
	return strings.EqualFold(v, "true")
}
