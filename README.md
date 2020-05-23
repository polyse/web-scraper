# web-scraper

This project consist of two parts: scrapper and listener. Spider walks through the web-page and sends payloads, that it found to the listener. Listener waiting for payloads and sends it to POLYSE database using [SDK](https://github.com/polyse/database-sdk). 

#### Installing

```bash
go get github.com/polyse/web-scrapper
```

#### Usage

1. Import package `import ws "github.com/polyse/web-scrapper"`
2. Install and start [RabbitMQ](https://www.rabbitmq.com/download.html).
3. Start [polySE database](https://github.com/polyse/database) on _<example_host>:<example_port>_
4. Run new spider like :
    ```bash
        cd cmd\daemon
        go build
        daemon.exe
    ``` 
5. Run new listener like :
    ```bash
        cd cmd\listener
        go build
        listener.exe
    ```
6. Send POST-message with auth Bearer token like :
    ```bash
        localhost:7171/start?url=http://go-colly.org
    ```
7. Enjoy results.

#### Credits

1. [go-colly](http://go-colly.org)
2. [surferua](https://github.com/jiusanzhou/surferua)
3. [rabbitmq](https://www.rabbitmq.com)
4. [wire](https://github.com/google/wire)