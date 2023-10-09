package HttpPackage

import (
    "strconv"
    "strings"
    "fmt"
)

const (
    FILE_NOTFOUND_PAYLOAD = "<!DOCTYPE HTML PUBLIC \"-//IETF//DTD HTML 2.0//EN\">\n<html>\n<head>\n   <title>404 Not Found</title>\n</head>\n<body>\n   <h1>Not Found</h1>\n   <p>The requested URL /t.html was not found on this server.</p>\n</body>\n</html>\n"
)

var (
    BASIC_GET_FILE_RESPONSE = HttpResponse{
        Version: "HTTP/1.1",
        Code: "200",
        Msg: "OK",
        Headers: map[string]string{
            "Connection": "Closed",
        },
        Content: nil,
    }
    FILE_NOTFOUND = HttpResponse {
        Version: "HTTP/1.1", Code: "404", Msg: "Not Found",
        Headers: map[string]string{
            "Content-Length": strconv.Itoa(len(FILE_NOTFOUND_PAYLOAD)),
            "Connection": "Closed",
            "Content-Type": "text/html; charset=iso-8859-1",
        },
        Content:  []byte(FILE_NOTFOUND_PAYLOAD),
    }
)

type HttpResponse struct {
    Version, Code, Msg string
    Headers map[string]string
    Content []byte
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
