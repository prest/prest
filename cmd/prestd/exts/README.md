# Extetion Arch

## In Go

```go
go build -o exts/lib/example.so -buildmode=plugin exts/hello.go
```

## In C

```c
gcc exts/helloc.c  -fPIC -shared -o exts/lib/example.so
```
