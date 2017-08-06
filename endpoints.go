package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Endpoints struct {
	Endpoints []string `json:"endpoints"`
}

func loadEndpoints(path string) []string {
	endpointsJson, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to load json: %s error: %v", path, err))
	}
	var e Endpoints
	err = json.Unmarshal(endpointsJson, &e)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse json: %v", err))
	}
	return e.Endpoints
}
