package main

import "net/http"

type Publish struct {
	endpoint string
}

func (p *Publish) Publish(kv *KeyValue){
	if p.endpoint != ""{
		http.Post()
	}
}
