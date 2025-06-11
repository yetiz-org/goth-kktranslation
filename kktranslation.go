package kktranslation

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/yetiz-org/goth-kklogger"
	"gopkg.in/yaml.v2"
)

var LangRootPath = "./resources/translation"
var TranslateFallback = true
var DefaultLang = "zh-tw"
var langMap = sync.Map{}
var langFiles []LangFile
var langLoadLock = sync.Mutex{}
var emptyLangFile = &LangFile{}
var fullLoaded = sync.Once{}

type LangFile struct {
	Version string            `yaml:"version"`
	Lang    string            `yaml:"lang"`
	Name    string            `yaml:"name"`
	Dict    map[string]string `yaml:"dict"`
}

func _LoadLangFile(lang string) *LangFile {
	lang = _LangNameNormalize(lang)
	if _IsDebug() {
		langMap.Delete(lang)
	}

	if l, f := langMap.Load(lang); !f {
		defer langLoadLock.Unlock()
		langLoadLock.Lock()
		if _, f := langMap.Load(lang); !f {
			langFile := &LangFile{}
			ml := func() string {
				if slang := strings.Split(lang, "-"); len(slang) > 1 {
					return slang[0]
				}

				return ""
			}()

			if data, err := ioutil.ReadFile(fmt.Sprintf("%s/%s.yaml", LangRootPath, lang)); err == nil {
				if err := yaml.Unmarshal(data, langFile); err != nil {
					kklogger.WarnJ("KKTranslation.LoadLangFile", err.Error())
					return nil
				}

				langMap.Store(lang, langFile)
				return langFile
			} else if ml != "" {
				if data, err := ioutil.ReadFile(fmt.Sprintf("%s/%s.yaml", LangRootPath, ml)); err == nil {
					if err := yaml.Unmarshal(data, langFile); err != nil {
						kklogger.WarnJ("KKTranslation.LoadLangFile", err.Error())
						return nil
					}

					langMap.Store(ml, langFile)
					return langFile
				}
			}
		}

		return nil
	} else {
		return l.(*LangFile)
	}
}

func LangFiles() []LangFile {
	if _IsDebug() {
		langLoadLock.Lock()
		langMap = sync.Map{}
		langFiles = []LangFile{}
		fullLoaded = sync.Once{}
		langLoadLock.Unlock()
	}

	fullLoaded.Do(func() {
		if files, e := ioutil.ReadDir(LangRootPath); e == nil {
			for _, file := range files {
				if !file.IsDir() {
					if langFile := _LoadLangFile(strings.Split(file.Name(), ".")[0]); langFile != nil {
						langFiles = append(langFiles, *langFile)
					}
				}
			}
		}
	})

	return langFiles
}

func GetLangFile(lang string) *LangFile {
	if langFile := _LoadLangFile(lang); langFile != nil {
		return langFile
	} else if langFile = _LoadLangFile(DefaultLang); langFile != nil {
		return langFile
	} else {
		return emptyLangFile
	}
}

func (l *LangFile) T(message string) string {
	if m, f := l.Dict[message]; f {
		return m
	}

	if ml := func() string {
		if slang := strings.Split(_LangNameNormalize(l.Lang), "-"); len(slang) > 1 {
			return slang[0]
		}

		return ""
	}(); ml != "" {
		if lf := _LoadLangFile(ml); lf != nil {
			return lf.T(message)
		}
	}

	if lf := GetLangFile(DefaultLang); TranslateFallback && lf != nil && l.Lang != lf.Lang {
		return lf.T(message)
	}

	return message
}

func _LangNameNormalize(lang string) string {
	return strings.ToLower(lang)
}

func _IsDebug() bool {
	return strings.ToUpper(os.Getenv("KKAPP_DEBUG")) == "TRUE"
}
