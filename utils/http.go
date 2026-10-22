package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxJSONBodySize 请求体上限（1MB）：控制面请求都是小对象，
// 限制读取量可防止大 body 造成内存峰值。
const maxJSONBodySize = 1 << 20

func ReadJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	if r.Body == nil {
		return errors.New("empty request body")
	}
	// 多读 1 字节用于判定是否超限，避免静默截断后产生晦涩的 JSON 解析错误
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodySize+1))
	if err != nil {
		return err
	}
	if len(body) > maxJSONBodySize {
		return errors.New("request body too large (max 1MB)")
	}
	return json.Unmarshal(body, v)
}

func WriteJSON(w http.ResponseWriter, errMsg error, data interface{}) {
	resp := &struct {
		ErrCode int
		ErrMsg  string
		Data    interface{}
	}{}

	if errMsg != nil {
		resp.ErrCode = -1
		resp.ErrMsg = errMsg.Error()
	} else {
		resp.ErrCode = 0
		resp.ErrMsg = ""
		resp.Data = data
	}

	buf, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("json marshal failed"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
}

func ParseRespJSON(r io.Reader, v interface{}) error {
	var resp struct {
		ErrCode int
		ErrMsg  string
		Data    interface{}
	}
	resp.Data = v

	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf, &resp)
	if err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return errors.New(resp.ErrMsg)
	}
	return nil
}
