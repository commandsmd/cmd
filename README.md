# `cmd`

## Run and organize code snippets in markdown files

#### `docs:update-demos`
```
docker run -it --rm -v $(pwd)/demos:/demos $(docker build -q -f ./record-demo.Dockerfile .) /demos/record-script.sh /demos/intro/intro.script /demos/intro/intro.cast
docker run -it --rm -v $(pwd)/demos:/demos $(docker build -q -f ./record-demo.Dockerfile .) asciinema upload /demos/intro/intro.cast
```

#### `code:test`
```
go test ./...
```
