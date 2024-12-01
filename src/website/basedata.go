package website

import (
	"fmt"
	"hsf/src/buildcss"
)

type BaseData struct {
	EsBuildSSEUrl string
}

func GetBaseData() BaseData {
	esbuildUrl := ""
	if buildcss.ActiveServerPort != 0 {
		esbuildUrl = fmt.Sprintf("localhost:%d", buildcss.ActiveServerPort)
	}
	return BaseData{
		EsBuildSSEUrl: esbuildUrl,
	}
}
