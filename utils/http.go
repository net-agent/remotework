package utils

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
)

func ReadJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
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
	w.Write(buf)
}

func ParseRespJSON(r io.Reader, v interface{}) error {
	var resp struct {
		ErrCode int
		ErrMsg  string
		Data    interface{}
	}
	resp.Data = v

	buf, err := ioutil.ReadAll(r)
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
