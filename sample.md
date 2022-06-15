# Top

## Second


#### `hi`
This is a useful command. It says hello!

``` bash
echo hi
```

## More depth

#### `greeting`

Another awesome command, but it cares about *who you are*. TODO add intelligence.

``` bash
export NAME
echo hi $NAME, nice to $1 you
```

#### `greeting2`
``` bash
echo hi $name, nice to $1 you
```

#### `greeting3`
``` bash
: ${NAME}
echo hi $NAME, nice to $1 you
```

## Other commands without help...

#### `pygreeting`
``` python
print("Hello from python")
```

#### `jsgreeting`
``` node
console.log("Hello from node")
```

## And a few other commands that have syntax errors...

#### `badjsgreeting`
``` node group=bad
console.log('Hello from node")
```

#### `badpygreeting`
``` python group=bad
print('Hello from python")
```

## What about running code in containers?

#### `python39match`
``` python image=python:3.9-slim
x = 401
match x:
    case 401 | 403 | 404:
        print("nope!")
    case _:
        print("ok")
```


#### `python310match`

Five great reasons to use it:
- one
- two
- three
- four
- there is no five

``` python image=python:3.10-slim
x = 401
match x:
    case 401 | 403 | 404:
        print("nope!")
    case _:
        print("ok")
```



| variable  | source |
| --------- | ----------- |
| SECRET    | op://vault/aws/secret_key_id |
