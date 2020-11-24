package main

// Hello plugin
func Hello() (ret string) {
	ret = "Hello plugin caller!"
	return
}

// go build -o exts/lib/hello.so -buildmode=plugin exts/hello.go
