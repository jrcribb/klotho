package main

import (
	"fmt"

	"github.com/klothoplatform/klotho/pkg/k2/model"
	"gopkg.in/yaml.v3"
)

func irCmd(filePath string) string {
	ir, err := model.ReadIRFile(filePath)
	if err != nil {
		return fmt.Sprintf("error reading IR file: %s", err)
	}

	res, err := yaml.Marshal(ir)
	if err != nil {
		return fmt.Sprintf("error marshalling IR: %s", err)
	}
	return string(res)
}
