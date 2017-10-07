package main

import "net/http"

type Publisher interface {
	Publish(value *KeyValue)
}

type Logic struct {
	publisher Publisher
}

func (l *Logic) Execute(c *http.Client) (map[string]int, error) {
	return l.testRequest(c)
}

func (l *Logic) testRequest(c *http.Client) (map[string]int, error) {
	resChan := make(chan *KeyValue)
	defer close(resChan)
	m := make(map[string]int)
	for _, endpoint := range endpoints {
		m[endpoint] = 0
	}

	for k := range m {
		go func(cl *http.Client, endpoint string, publisher Publisher) {
			response, err := cl.Get(endpoint)
			if err != nil {
				res := &KeyValue{endpoint, -1}
				publisher.Publish(res)
				resChan <- res
			} else {
				res := &KeyValue{endpoint, response.StatusCode}
				if response.StatusCode != 200 {
					publisher.Publish(res)
				}
				resChan <- res
			}
		}(c, k, l.publisher)
	}
	for i := 0; i < len(m); i++ {
		select {
		case result := <-resChan:
			m[result.Key] = result.Value
		}
	}

	return m, nil
}
