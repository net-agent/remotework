package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"

	"github.com/BurntSushi/toml"
)

// LoadJSONFile 加载json文件到对象里
func LoadJSONFile(pathname string, v interface{}) error {
	buf, err := os.ReadFile(pathname)
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

// FirstString 返回第一个长度大于0的字符串
func FirstString(strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return ""
}

// ResolveConfigFile 解析配置文件路径，支持 fallback 到 config.json / config.toml
func ResolveConfigFile(configName string) (string, error) {
	if FileExist(configName) {
		return configName, nil
	}
	dir := path.Dir(configName)
	configJson := path.Join(dir, "config.json")
	configToml := path.Join(dir, "config.toml")
	if FileExist(configJson) {
		return configJson, nil
	}
	if FileExist(configToml) {
		return configToml, nil
	}
	return "", fmt.Errorf("config file not found: tried '%v', '%v', '%v'", configName, configJson, configToml)
}
