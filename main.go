package main
// TODO: auto reload [x]
//       TODO: learn websocket [x]
//       TODO: inject code into html entry file to run websocket [x]
// TODO: CLEAN UP THIS DUMP MESS T_T [x]
// TODO: don't reload when unrelated files get editted [ ]
// TODO: maybe figure out why golang/sha256.Sum256 not working properpy (or maybe os.ReadFile not working) [ ]
import(
    http "livesv/HttpPackage"
    cache "livesv/FileCache"
    ws "livesv/WebSocket"
    "sync/atomic"
    "strconv"
    "runtime"
    "path/filepath"
    "strings"
    "os"
    "fmt"
    "net"
)

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
)

var (
    entry_file, root_dir string
    path_seperator string
    websocket_channel, file_cache_channel = make(chan string), make(chan string)
    default_browser_opener string
    has_websocket atomic.Bool
)

func main() {
    os_independent_set_args()
    fmt.Printf("Opener: %s\nSep: %s\n", default_browser_opener, path_seperator)
    if len(os.Args) <= 1 {
        fmt.Println("USAGE: progname `html file`") 
        os.Exit(1)
    }
    root_dir = filepath.Dir(os.Args[1])
    entry_file = filepath.Base(os.Args[1])
    fmt.Printf("[INFO] Start server with file `%s`\n", entry_file)
    fmt.Printf("[INFO] Root directory `%s`\n", root_dir)
    server, err := net.Listen("tcp", SERVER_ADDR)
    Check_err(err, true, "[INFO] Can't create server")
    defer server.Close()
    fmt.Println("Server on:", server.Addr().String())
    fmt.Println("Start listening...")

    // open in browser
    var proc_attr *os.ProcAttr = new(os.ProcAttr)
    _, err = os.StartProcess(default_browser_opener,
                            []string{default_browser_opener,
                                     fmt.Sprintf("http://%s/", SERVER_ADDR)}, proc_attr) // start default broser
    Check_err(err, true, "Can't start `%s`\n", default_browser_opener)

    go cache.Update_cache_files(file_cache_channel, func(_ string) {
        if has_websocket.Load() { websocket_channel <- "RELOAD" }
    })
    for {
        client, err := server.Accept()
        if Check_err(err, false, "Can't accep client") { continue }
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

func inject_websocket(file_content []byte) []byte {
    content_str := string(file_content)
    end_html_tag_index := strings.LastIndex(content_str, "</html>")
    if end_html_tag_index == -1 {
        fmt.Println("Write some proper html bro -_-")
        return file_content
    }
    return []byte(content_str[:end_html_tag_index] + WEBSOCKET_INJECT_CODE + content_str[end_html_tag_index:])
}


func handle_websocket(client net.Conn, request *http.HttpRequest) {
    response := ws.Build_websocket_accept(request.Headers["Sec-WebSocket-Key"])
    client.Write(response.Build())
    fmt.Println("[INFO] Successfully connected")
    for {
        msg := <-websocket_channel
        if msg == "CLOSE" {
            _, err := client.Write(ws.CLOSE_FRAME)
            Check_err(err, false, "Can't send quit message")
            fmt.Println("[INFO] Send `quit` message to client")
            break;
        } else if msg == "RELOAD" {
            n, err := client.Write(ws.Build_websocket_frame_msg(msg))
            Check_err(err, false, "Can't send message")
            fmt.Printf("[INFO] Sent message `%s` to client\n", msg, n)
        }
    }
}

func handle_http(client net.Conn, request *http.HttpRequest) {
    switch request.Req_type {
    case "GET":
        var response http.HttpResponse
        fmt.Printf("[INFO] Client request for `%s`\n", request.File_path)
        file_path := root_dir
        if request.File_path == "/" { file_path += path_seperator + entry_file } else { file_path += request.File_path }
        file_content := cache.Get_file_content(file_path)
        if file_content != nil {
            if !cache.Is_cached(file_path) { file_cache_channel <- file_path }
            response = http.BASIC_GET_FILE_RESPONSE
            file_ext := filepath.Ext(file_path)
            response.Headers["Content-type"] = http.CONTENT_TYPES[file_ext]
            response.Headers["Content-Length"] = strconv.Itoa(len(file_content))
            response.Content = file_content
            if file_ext == ".html" {
                response.Headers["Content-Length"] = strconv.Itoa(len(file_content) + len(WEBSOCKET_INJECT_CODE))
                response.Content = inject_websocket(file_content)
            }
            fmt.Printf("[INFO] Send file `%s` %d bytes to client\n", file_path, len(file_content))
        } else {
            response = http.FILE_NOTFOUND
        }
        client.Write(response.Build())
    default:
        fmt.Println("Unimplemented")
        os.Exit(1)
    }
}

func handle_client(client net.Conn) {
    defer client.Close()
    buffer := make([]byte, 1024)
    n, err := client.Read(buffer)
    if Check_err(err, false, "Can't read from client") { return }
    request := http.Parse_request(string(buffer[:n]))
    if request.Headers["Connection"] == "Upgrade" && request.Headers["Upgrade"] == "websocket" {
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
}

func Check_err(err error, fatal bool, info ...string) bool {
    if err != nil {
        var msg_builder strings.Builder
        if fatal { msg_builder.WriteString("[ERROR] ") } else { msg_builder.WriteString("[WARNING] ") }
        msg_builder.WriteString(err.Error())
        msg_builder.WriteString("\n")
        for _, v := range info {
            msg_builder.WriteString("\t [INFO] ")
            msg_builder.WriteString(v)
        }
        if fatal {
            fmt.Println(msg_builder.String())
            os.Exit(1)
        } else {
            fmt.Println(msg_builder.String())
        }
        return true;
    }
    return false;
}
