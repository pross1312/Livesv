package HttpPackage

import (
    "strconv"
    "strings"
    "fmt"
)

const (
    FILE_NOTFOUND_PAYLOAD = "<!DOCTYPE HTML PUBLIC \"-//IETF//DTD HTML 2.0//EN\">\n<html>\n<head>\n   <title>404 Not Found</title>\n</head>\n<body>\n   <h1>Not Found</h1>\n   <p>The requested URL /t.html was not found on this server.</p>\n</body>\n</html>\n"
)

type HttpResponse struct {
    Version, Code, Msg string
    Headers map[string]string
    Content []byte
}

func Make_basic_ok(res *HttpResponse) {
    res.Version = "HTTP/1.1"
    res.Code = "200"
    res.Msg = "OK"
    if res.Headers == nil { res.Headers = make(map[string]string) }
    res.Headers["Connection"] = "Closed"
}

func Make_file_not_found(res *HttpResponse) {
    res.Version = "HTTP/1.1"
    res.Code = "404"
    res.Msg = "Not Found"
    if res.Headers == nil { res.Headers = make(map[string]string) }
    res.Headers["Content-Length"] = strconv.Itoa(len(FILE_NOTFOUND_PAYLOAD))
    res.Headers["Connection"] = "Closed"
    res.Headers["Content-Type"] = "text/html; charset=iso-8859-1"
    res.Content = []byte(FILE_NOTFOUND_PAYLOAD)
}

func (res HttpResponse) print() {
    fmt.Printf("Version: %s\nCode: %s\nMessage: %s\n", res.Version, res.Code, res.Msg)
    for k, v := range res.Headers {
        fmt.Printf("%s: %s\n", k, v)
    }
    fmt.Printf("-----------------------------------------------\n%s\n", res.Content)
}

func (res *HttpResponse) Build() []byte {
    var builder strings.Builder
    builder.WriteString(res.Version)
    builder.WriteString(" ")
    builder.WriteString(res.Code)
    builder.WriteString(" ")
    builder.WriteString(res.Msg)
    builder.WriteString("\r\n")
    for k, v := range res.Headers {
        builder.WriteString(k)
        builder.WriteString(": ")
        builder.WriteString(v)
        builder.WriteString("\r\n")
    }
    builder.WriteString("\r\n")
    builder.Write(res.Content)
    return []byte(builder.String())
}
