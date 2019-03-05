# joinin

### Build
```bash
go build .
``` 
   
### Deploy Server
```bash
satellite -server=true -port=80
```

### Deploy Client
```bash
satellite -server=false -remote="127.0.0.1:80"
```

