package main
// TODO: auto reload
import(
    "runtime"
    "path/filepath"
    "strings"
    "os"
    "fmt"
    "net"
)

type HtmlRequest struct {
    req_type, file_path, version string
    headers map[string]string
    content string
}
type HtmlResponse struct {
    version, code, msg string
    headers map[string]string
    content []byte
}

func (res HtmlResponse) build() []byte {
    var builder strings.Builder
    builder.WriteString(res.version)
    builder.WriteString(" ")
    builder.WriteString(res.code)
    builder.WriteString(" ")
    builder.WriteString(res.msg)
    builder.WriteString("\r\n")
    for k, v := range res.headers {
        builder.WriteString(k)
        builder.WriteString(": ")
        builder.WriteString(v)
        builder.WriteString("\r\n")
    }
    builder.WriteString("\r\n")
    builder.Write(res.content)
    return []byte(builder.String())
}

const(
    SERVER_ADDR = "localhost:13123"
    FILE_NOTFOUND = "HTTP/1.1 404 Not Found\r\nDate: Wed, 10 Oct 2023 00:16:00 GMT\r\nContent-Length: 230\r\nConnection: Closed\r\nContent-Type: text/html; charset=iso-8859-1\r\n\r\n<!DOCTYPE HTML PUBLIC \"-//IETF//DTD HTML 2.0//EN\">\n<html>\n<head>\n   <title>404 Not Found</title>\n</head>\n<body>\n   <h1>Not Found</h1>\n   <p>The requested URL /t.html was not found on this server.</p>\n</body>\n</html>\n"
    TEST_FILE_PATH = "/home/dvtuong/programming/odin-project/index.html"
    OK_GET_FILE_FORMAT = "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: Closed\r\nContent-Type: text/html; charset=iso-8859-1\r\n\r\n%s"
)

var(
    entry_file, root_dir string
    content_types = map[string]string{
        ".html": "text/html",
        ".css": "text/css",
        ".js": "text/javascript",
    }
    basic_get_file_response = HtmlResponse{
        version: "HTTP/1.1",
        code: "200",
        msg: "OK",
        headers: map[string]string{
            "Connection": "Closed",
        },
        content: nil,
    }
    path_seperator string
)

func main() {
    switch runtime.GOOS {
    case "windows":
        path_seperator = "\\"
    default:
        path_seperator = "/"
    }
    if len(os.Args) <= 1 {
        fmt.Println("USAGE: progname `html file`") 
        os.Exit(1)
    }
    root_dir = filepath.Dir(os.Args[1])
    entry_file = filepath.Base(os.Args[1])
    fmt.Printf("[INFO] Start server with file `%s`\n", entry_file)
    fmt.Printf("[INFO] Root directory `%s`\n", root_dir)
    server, err := net.Listen("tcp", SERVER_ADDR)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] Can't create server, %s\n", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("Server on:", server.Addr().String())
    fmt.Println("Start listening...")
    start_browser("http://127.0.0.1:13123/")
    for {
        client, err := server.Accept()
        if err != nil {
            fmt.Fprintln(os.Stderr, "[ERROR] accepting client: ", err.Error())
            continue
        }
        go process_client(client)
    }

}

func start_browser(addr string) {
    switch runtime.GOOS {
    case "windows":
        fmt.Fprintln(os.Stderr, "[ERROR] Unsupported platform")
        os.Exit(1)
    case "darwin":
        fmt.Fprintln(os.Stderr, "[ERROR] Unsupported platform")
        os.Exit(1)
    case "linux":
        var proc_attr *os.ProcAttr = new(os.ProcAttr)
        _, err := os.StartProcess("/usr/bin/xdg-open", []string{"/usr/bin/xdg-open", addr}, proc_attr) // start default broser
        if err != nil {
            fmt.Fprintf(os.Stderr, "[ERROR] Can't start xdg-open, error: %s\n", err.Error())
            os.Exit(1)
        }
    default:
        fmt.Fprintln(os.Stderr, "[ERROR] Unsupported platform")
        os.Exit(1)
    }
}

func (req HtmlRequest) print() {
    fmt.Printf("Type: %s\nFile: %s\nVersion: %s\n", req.req_type, req.file_path, req.version)
    for k, v := range req.headers {
        fmt.Printf("%s: %s\n", k, v)
    }
    fmt.Printf("-----------------------------------------------%s\n", req.content)
}

func new_request() HtmlRequest {
    var req HtmlRequest
    req.headers = make(map[string]string)
    return req
}

func parse_request(request string) HtmlRequest {
    result := new_request()
    // parse first line (type, file, msg)
    end := strings.Index(request, "\r\n")
    line := request[:end]
    line_data := strings.Split(line, " ")
    assert(len(line_data) == 3, "[ERROR] Can't parse " + line)
    result.req_type = line_data[0]
    result.file_path = line_data[1]
    result.version = line_data[2]
    // parse headers ... (Connection: keep-alive)
    for {
        request = request[end+2:]
        end = strings.Index(request, "\r\n")
        line = request[:end]
        if len(line) == 0 {break} // end of headers 
        sep_index := strings.Index(line, ": ")
        assert(sep_index != -1, "[ERROR] Can't parse " + line)
        result.headers[line[:sep_index]] = line[sep_index+2:]
    }
    result.content = request[end+2:]
    return result
}

func assert(value bool, args ...string) {
    if (!value) {
        fmt.Fprintln(os.Stderr, args)
        os.Exit(1)
    }
}

func get_file_content(file_path string) []byte {
    content, err := os.ReadFile(file_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] Can't read file %s\n", file_path)
        return nil
    }
    return content
}

func process_client(client net.Conn) {
    buffer := make([]byte, 1024)
    n, err := client.Read(buffer)
    assert(err == nil, "[ERROR] Can't read from client")
    request := parse_request(string(buffer[:n]))
    switch request.req_type {
    case "GET":
        fmt.Printf("[INFO] Client request for `%s`\n", request.file_path)
        file_path := root_dir
        if request.file_path == "/" { file_path += path_seperator + entry_file } else { file_path += request.file_path }
        response := basic_get_file_response
        file_content := get_file_content(file_path)
        if file_content != nil {
            response.headers["content-type"] = content_types[filepath.Ext(file_path)]
            response.headers["content-length"] = string(len(file_content))
            response.content = file_content
            client.Write(response.build())
            fmt.Printf("[INFO] Send file `%s` %d bytes to client\n", file_path, len(file_content))
        }
    default:
        fmt.Println("Unimplemented")
        os.Exit(1)
    }
    client.Close()
}
