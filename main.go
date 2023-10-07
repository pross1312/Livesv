package main
// TODO: auto reload [x]
//       TODO: learn websocket [x]
//       TODO: inject code into html entry file to run websocket [x]
// TODO: CLEAN UP THIS DUMP MESS T_T [ ]
// TODO: don't reload when unrelated files get editted [ ]
// TODO: maybe figure out why golang/sha256.Sum256 not working properpy (or maybe os.ReadFile not working) [ ]
import(
    "log"
    "sync/atomic"
    "strconv"
    "time"
    "encoding/base64"
    // "crypto/sha256"
    "crypto/sha1"
    "runtime"
    "path/filepath"
    "strings"
    "os"
    "os/exec"
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
    last_hash string
}
type FilePath = string
type FileCache = map[FilePath]FileCacheEntry

const (
    WEBSOCKET_INJECT_CODE = `
    <script>
        if ('WebSocket' in window) {
            let protocol = window.location.protocol === "http:" ? "ws://" : "wss://"
            let address = protocol + window.location.host + window.location.pathname
            let socket = new WebSocket(address)
            socket.onmessage = function(msg) {
                if (msg.data === "RELOAD") {
                    window.location.reload()
                    console.log("RELOADED")
                }
                else {
                    console.log("Unhandled")
                    console.log(msg)
                }
            }
        }
        else {
            alert("Browser does not support web socket, can't hotreload")
        }
    </script>
`
    SERVER_ADDR = "localhost:13123"
    TEST_FILE_PATH = "/home/dvtuong/programming/odin-project/index.html"
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
    websocket_channel, file_cache_channel = make(chan string), make(chan string)
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

    go update_cache_files(file_cache_channel)
    for {
        client, err := server.Accept()
        if err != nil {
            fmt.Fprintln(os.Stderr, "[ERROR] accepting client: ", err.Error())
            continue
        }
        go handle_client(client)
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
                out, err := exec.Command("sha256sum", k).Output()
                assert(err == nil)
                out_str := string(out)
                new_hash := out_str[:strings.Index(out_str, "  ")]
                if new_hash != v.last_hash {
                    fmt.Printf("%s\n", new_hash)
                    fmt.Printf("%s\n", v.last_hash)
                    if v.last_hash != "" && has_websocket.Load() { websocket_channel <- "RELOAD" } // reload if web socket is ready and not first time add file to cache
                    fmt.Printf("[INFO] Updated sha512 for file %s\n", k)
                }
                files_cache[k] = FileCacheEntry{
                    last_modified: *last_modified,
                    last_hash: new_hash,
                }
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

func inject_websocket(file_content []byte) []byte {
    content_str := string(file_content)
    end_html_tag_index := strings.LastIndex(content_str, "</html>")
    assert(end_html_tag_index != -1, "Write some proper html bro -_-")
    return []byte(content_str[:end_html_tag_index] + WEBSOCKET_INJECT_CODE + content_str[end_html_tag_index:])
}


func handle_websocket(client net.Conn, request *HttpRequest) {
    response := build_websocket_accept(request.headers["Sec-WebSocket-Key"])
    client.Write(response.build())
    fmt.Println("[INFO] Successfully connected")
    for {
        msg := <-websocket_channel
        if msg == "CLOSE" {
            _, err := client.Write([]byte{0b10000000 | 0x8, 0})
            check_err(err, false, "Can't send quit message")
            fmt.Println("[INFO] Send `quit` message to client")
            break;
        } else if msg == "RELOAD" {
            n, err := client.Write(build_websocket_frame_msg(msg))
            check_err(err, false, "[INFO] Can't send message")
            fmt.Printf("[INFO] Sent message `%s` to client\n", msg, n)
        }
    }
}

func handle_http(client net.Conn, request *HttpRequest) {
    switch request.req_type {
    case "GET":
        var response HttpResponse
        fmt.Printf("[INFO] Client request for `%s`\n", request.file_path)
        file_path := root_dir
        if request.file_path == "/" { file_path += path_seperator + entry_file } else { file_path += request.file_path }
        file_content := get_file_content(file_path)
        if file_content != nil {
            if _, found := files_cache[file_path]; !found {
                file_cache_channel <- file_path
            }
            response = basic_get_file_response
            file_ext := filepath.Ext(file_path)
            response.headers["Content-type"] = content_types[file_ext]
            response.headers["Content-Length"] = strconv.Itoa(len(file_content))
            response.content = file_content
            if file_ext == ".html" {
                response.headers["Content-Length"] = strconv.Itoa(len(file_content) + len(WEBSOCKET_INJECT_CODE))
                response.content = inject_websocket(file_content)
            }
            fmt.Printf("[INFO] Send file `%s` %d bytes to client\n", file_path, len(file_content))
        } else {
            response = FILE_NOTFOUND
        }
        client.Write(response.build())
    default:
        fmt.Println("Unimplemented")
        os.Exit(1)
    }
}

func handle_client(client net.Conn) {
    buffer := make([]byte, 1024)
    n, err := client.Read(buffer)
    assert(err == nil, "[ERROR] Can't read from client")
    request := parse_request(string(buffer[:n]))
    if request.headers["Connection"] == "Upgrade" && request.headers["Upgrade"] == "websocket" {
        if has_websocket.Load() {
            websocket_channel <- "CLOSE"
            for has_websocket.Load() {}
        }
        has_websocket.Store(true)
        handle_websocket(client, &request)
        has_websocket.Store(false)
    } else {
        handle_http(client, &request)
    }
    fmt.Println("[INFO] Client closed")
    client.Close()
}

func check_err(err error, fatal bool, info ...string) bool {
    if err != nil {
        var msg_builder strings.Builder
        if fatal { msg_builder.WriteString("[ERROR] ") } else { msg_builder.WriteString("[WARNING] ") }
        msg_builder.WriteString(err.Error())
        msg_builder.WriteString("\n")
        for _, v := range info {
            msg_builder.WriteString("\t [INFO] ")
            msg_builder.WriteString(v)
        }
        if fatal { log.Fatalln(msg_builder.String()) } else { log.Println(msg_builder.String()) }
        return true;
    }
    return false;
}
