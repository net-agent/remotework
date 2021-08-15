package utils

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
)

// LoadJSONFile 加载json文件到对象里
func LoadJSONFile(pathname string, v interface{}) error {
	buf, err := ioutil.ReadFile(pathname)
	if err != nil {
		return err
	}

	// 去掉双斜杠注释
	re := regexp.MustCompile(`(^|\n)\s*\/\/.*`)
	jsonBuf := re.ReplaceAll(buf, nil)

	return json.Unmarshal(jsonBuf, v)
}

// LoadTomlFile 加载Toml文件到对象里
func LoadTomlFile(pathname string, v interface{}) error {
	_, err := toml.DecodeFile(pathname, v)
	return err
}

func FileExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Fatal("load file error: ", err)
	return false
}
