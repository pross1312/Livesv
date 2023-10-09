package HttpPackage
import (
    "strings"
    "os"
    "fmt"
)
var (
    CONTENT_TYPES = map[string]string{
        ".html": "text/html",
        ".css": "text/css",
        ".js": "text/javascript",
    }
)

type HttpRequest struct {
    Req_type, File_path, Version string
    Headers map[string]string
    Content string
}
func Parse_request(request string) *HttpRequest {
    result := new_request()
    // parse first line (type, file, msg)
    end := strings.Index(request, "\r\n")
    if end == -1 { os.Exit(1) }
    line := request[:end]
    line_data := strings.Split(line, " ")
    if len(line_data) != 3 {
        fmt.Printf("[ERROR] Can't parse %s\n", line)
        os.Exit(1)
    }
    result.Req_type = line_data[0]
    result.File_path = line_data[1]
    result.Version = line_data[2]
    // parse headers ... (Connection: keep-alive)
    for {
        request = request[end+2:]
        end = strings.Index(request, "\r\n")
        line = request[:end]
        if len(line) == 0 {break} // end of headers 
        sep_index := strings.Index(line, ": ")
        if sep_index == -1 {
            fmt.Printf("[ERROR] Can't parse %s\n", line)
            os.Exit(1)
        }
        result.Headers[line[:sep_index]] = line[sep_index+2:]
    }
    result.Content = request[end+2:]
    return result
}
func new_request() *HttpRequest {
    var req = new(HttpRequest)
    req.Headers = make(map[string]string)
    return req
}
func (req HttpRequest) print() {
    fmt.Printf("Type: %s\nFile: %s\nVersion: %s\n", req.Req_type, req.File_path, req.Version)
    for k, v := range req.Headers {
        fmt.Printf("%s: %s\n", k, v)
    }
    fmt.Printf("-----------------------------------------------\n%s\n", req.Content)
}


