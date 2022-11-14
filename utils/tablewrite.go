package utils

import (
	"io"
	"reflect"

	"github.com/olekukonko/tablewriter"
)

func RenderAsciiTable(w io.Writer, arr interface{}, headers []string, mapfn func(d interface{}, index int) []string) {
	t := tablewriter.NewWriter(w)
	t.SetHeader(headers)

	switch reflect.TypeOf(arr).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(arr)
		for i := 0; i < s.Len(); i++ {
			t.Append(mapfn(s.Index(i).Interface(), i))
		}
	}

	t.Render()
}
