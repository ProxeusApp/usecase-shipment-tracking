# Single Binary UI


> ### Usage
>> ```sh
>># ./server --help
>># Usage of ./server:
>>#   -h string
>>#     	Host (default "127.0.0.1")
>>#   -p string
>>#     	Port (default "58084")
>> 
>> $ ./server
>> ```
>

## **GO Build all cmd's at once**
>```sh
>$ go run server_exe/embed/make/main.go && rm ./bindata.go && GOOS=linux GOARCH=arm GOARM=7 go build -o server server_exe/main.go
>```