package main
// TODO: auto reload [ ]
//       TODO: learn websocket [x]
//       TODO: inject code into html entry file to run websocket [ ]
// TODO: CLEAN UP THIS DUMP MESS T_T
import(
    "sync/atomic"
    "strconv"
    "time"
    "encoding/base64"
    "crypto/sha512"
    "crypto/sha1"
    "runtime"
    "path/filepath"
    "strings"
    "os"
    "fmt"
    "net"
)

type HttpRequest struct {
    req_type, file_path, version string
    headers map[string]string
    content string
}

type HttpResponse struct {
    version, code, msg string
    headers map[string]string
    content []byte
}

func (req HttpResponse) print() {
    fmt.Printf("Version: %s\nCode: %s\nMessage: %s\n", req.version, req.code, req.msg)
    for k, v := range req.headers {
        fmt.Printf("%s: %s\n", k, v)
    }
    fmt.Printf("-----------------------------------------------%s\n", req.content)
}

func (res HttpResponse) build() []byte {
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

type FileCacheEntry struct {
    last_modified time.Time
    last_sha512 [sha512.Size]byte
}
type FilePath = string
type FileCache = map[FilePath]FileCacheEntry


const (
    SERVER_ADDR = "localhost:13123"
    TEST_FILE_PATH = "/home/dvtuong/programming/odin-project/index.html"
    OK_GET_FILE_FORMAT = "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: Closed\r\nContent-Type: text/html; charset=iso-8859-1\r\n\r\n%s"
    FILE_NOTFOUND_PAYLOAD = "<!DOCTYPE HTML PUBLIC \"-//IETF//DTD HTML 2.0//EN\">\n<html>\n<head>\n   <title>404 Not Found</title>\n</head>\n<body>\n   <h1>Not Found</h1>\n   <p>The requested URL /t.html was not found on this server.</p>\n</body>\n</html>\n"
)

var (
    FILE_NOTFOUND = HttpResponse{
        version: "HTTP/1.1", code: "404", msg: "Not Found",
        headers: map[string]string{
            "Content-Length": strconv.Itoa(len(FILE_NOTFOUND_PAYLOAD)),
            "Connection": "Closed",
            "Content-Type": "text/html; charset=iso-8859-1",
        },
        content:  []byte(FILE_NOTFOUND_PAYLOAD),
    }
    log_file *os.File
    entry_file, root_dir string
    content_types = map[string]string{
        ".html": "text/html",
        ".css": "text/css",
        ".js": "text/javascript",
    }
    basic_get_file_response = HttpResponse{
        version: "HTTP/1.1",
        code: "200",
        msg: "OK",
        headers: map[string]string{
            "Connection": "Closed",
        },
        content: nil,
    }
    path_seperator string
    files_cache = make(FileCache)
    websocket_channel = make(chan string)
    default_browser_opener string
    has_websocket atomic.Bool
)

func main() {
    os_independent_set_args()
    fmt.Printf("Opener: %s\nSep: %s\n", default_browser_opener, path_seperator)
    log_file, _ = os.Open("log")
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

    // open in browser
    var proc_attr *os.ProcAttr = new(os.ProcAttr)
    _, err = os.StartProcess(default_browser_opener,
                            []string{default_browser_opener,
                                     fmt.Sprintf("http://%s/", SERVER_ADDR)}, proc_attr) // start default broser
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] Can't start `%s`, error: %s\n", default_browser_opener, err.Error())
        os.Exit(1)
    }

    ch := make(chan string)
    go update_cache_files(ch)
    go websocket_server("localhost:9999", websocket_channel)
    for {
        client, err := server.Accept()
        if err != nil {
            fmt.Fprintln(os.Stderr, "[ERROR] accepting client: ", err.Error())
            continue
        }
        go process_client(ch, client)
    }

}
func os_independent_set_args() {
    switch runtime.GOOS {
    case "windows":
        path_seperator = "\\"
        default_browser_opener = "explorer"
    case "darwin":
        fmt.Fprintln(os.Stderr, "[ERROR] Unsupported platform")
        os.Exit(1)
    case "linux":
        default_browser_opener = "/usr/bin/xdg-open"
        path_seperator = "/"
    default:
        fmt.Fprintln(os.Stderr, "[ERROR] Unsupported platform")
        os.Exit(1)
    }
}

func (req HttpRequest) print() {
    fmt.Printf("Type: %s\nFile: %s\nVersion: %s\n", req.req_type, req.file_path, req.version)
    for k, v := range req.headers {
        fmt.Printf("%s: %s\n", k, v)
    }
    fmt.Printf("-----------------------------------------------%s\n", req.content)
}

func new_request() HttpRequest {
    var req HttpRequest
    req.headers = make(map[string]string)
    return req
}

func parse_request(request string) HttpRequest {
    result := new_request()
    // parse first line (type, file, msg)
    end := strings.Index(request, "\r\n")
    if end == -1 { os.Exit(1) }
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

func get_last_modified(file_path string) *time.Time {
    file, err := os.Open(file_path)
    if err != nil { return nil }
    defer file.Close()
    info, err := file.Stat()
    assert(err == nil, "[ERROR] Can't stat file " + file_path)
    result := new(time.Time)
    *result = info.ModTime()
    return result
}

func update_cache_files(ch chan string) {
    for {
        select {
        case x, ok := <-ch:
            if ok {
                files_cache[x] = FileCacheEntry{}
                fmt.Printf("[INFO] Cache file `%s`\n", x)
            } else {
                fmt.Println("[INFO] Channel closed!")
            }
        default:
        }
        for k, v := range files_cache {
            last_modified := get_last_modified(k)
            if last_modified != nil && !v.last_modified.Equal(*last_modified) {
                v.last_modified = *last_modified
                new_sha512 := sha512.Sum512(get_file_content(k))
                if new_sha512 != v.last_sha512 {
                    v.last_sha512 = new_sha512
                    if has_websocket.Load() { websocket_channel <- "RELOAD" }
                    fmt.Printf("[INFO] Updated sha512 for file %s\n", k)
                }
                files_cache[k] = v
                fmt.Printf("[INFO] Updated modified time for file %s\n", k)
            }
        }
    }
}
// HTTP/1.1 101 Switching Protocols
// Upgrade: websocket
// Connection: Upgrade
// Sec-WebSocket-Accept: rG8wsswmHTJ85lJgAE3M5RTmcCE=

// Websocket Frame format:
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-------+-+-------------+-------------------------------+
// |F|R|R|R| opcode|M| Payload len |    Extended payload length    |
// |I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
// |N|V|V|V|       |S|             |   (if payload len==126/127)   |
// | |1|2|3|       |K|             |                               |
// +-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
// |     Extended payload length continued, if payload len == 127  |
// + - - - - - - - - - - - - - - - +-------------------------------+
// |                               |Masking-key, if MASK set to 1  |
// +-------------------------------+-------------------------------+
// | Masking-key (continued)       |          Payload Data         |
// +-------------------------------- - - - - - - - - - - - - - - - +
// :                     Payload Data continued ...                :
// + - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
// |                     Payload Data continued ...                |
// +---------------------------------------------------------------+
// 
func build_websocket_accept(key string) HttpResponse {
    const magical_suffix = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
    var result HttpResponse
    result.headers = make(map[string]string)
    accept_key_sha1 := sha1.Sum([]byte(key + magical_suffix))
    accept_key_base64 := base64.StdEncoding.EncodeToString(accept_key_sha1[:])
    result.version = "HTTP/1.1"
    result.code = "101"
    result.msg = "Switching Protocols"
    result.headers = map[string]string{
        "Upgrade": "websocket",
        "Connection": "Upgrade",
        "Sec-WebSocket-Accept": accept_key_base64,
    }
    return result
}

func build_websocket_frame_msg(msg string) []byte {
    if len(msg) < 126 {
        return append([]byte{0x81, byte(len(msg))}, msg...)
    } else {
        fmt.Fprintf(os.Stderr, "[ERROR] Umimplemented\n")
        os.Exit(1)
    }
    return nil
}

func websocket_server(address string, msg_ch chan string) {
    var _ int64
    server, err := net.Listen("tcp", address)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR Web socket server] Can't create server, %s\n", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    for {
        client, err := server.Accept()
        if err != nil {
            fmt.Fprintln(os.Stderr, "[ERROR Web socket server] accepting client: ", err.Error())
            os.Exit(1)
        }
        defer client.Close()
        // handshake
        buffer := make([]byte, 1024)
        n, err := client.Read(buffer)
        assert(err == nil, "[ERROR] Can't read from client")
        request := parse_request(string(buffer[:n]))
        if key, found := request.headers["Sec-WebSocket-Key"]; found {
            response := build_websocket_accept(key)
            client.Write(response.build())
        }
        fmt.Println("[INFO] Web socket successfully connected")
        has_websocket.Store(true)
        for {
            msg := <-msg_ch
            n, err := client.Write(build_websocket_frame_msg(msg))
            if err != nil {
                fmt.Fprintf(os.Stderr, "[WARN] Can't send %s to client, error: %s\n", msg, err.Error())
                fmt.Println("[INFO] Web socket shut down")
                has_websocket.Store(false)
                break
            }
            fmt.Printf("[INFO] Sent message `%s`, %d bytes to client", msg, n)
        }
        has_websocket.Store(false)
    }
}

func process_client(ch chan string, client net.Conn) {
    buffer := make([]byte, 1024)
    n, err := client.Read(buffer)
    assert(err == nil, "[ERROR] Can't read from client")
    request := parse_request(string(buffer[:n]))
    switch request.req_type {
    case "GET":
        var response HttpResponse
        fmt.Printf("[INFO] Client request for `%s`\n", request.file_path)
        file_path := root_dir
        if request.file_path == "/" { file_path += path_seperator + entry_file } else { file_path += request.file_path }
        file_content := get_file_content(file_path)
        if _, found := files_cache[file_path]; !found {
            ch <- file_path
        }
        if file_content != nil {
            response = basic_get_file_response
            response.headers["Content-type"] = content_types[filepath.Ext(file_path)]
            response.headers["Content-Length"] = strconv.Itoa(len(file_content))
            response.content = file_content
            fmt.Printf("[INFO] Send file `%s` %d bytes to client\n", file_path, len(file_content))
        } else {
            response = FILE_NOTFOUND
        }
        client.Write(response.build())
    default:
        fmt.Println("Unimplemented")
        os.Exit(1)
    }
    client.Close()
}
