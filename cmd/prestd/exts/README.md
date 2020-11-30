# Extetion Arch

## In Go

```shell
go build -o exts/lib/example.so -buildmode=plugin exts/hello.go
```

## In C

```shell
gcc exts/helloc.c  -fPIC -shared -o exts/lib/example.so
```
