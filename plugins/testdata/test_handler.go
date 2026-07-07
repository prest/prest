package main
import "encoding/json"

type Response struct{ MSG string `json:"msg"` }

func GETFooBarHandler() (ret string) {
	b, _ := json.Marshal(Response{MSG: "foobar"})
	return string(b)
}
