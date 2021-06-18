package utils

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
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
