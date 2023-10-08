package main
// TODO: auto reload [x]
//       TODO: learn websocket [x]
//       TODO: inject code into html entry file to run websocket [x]
// TODO: CLEAN UP THIS DUMP MESS T_T [x]
// TODO: maybe figure out why golang/sha256.Sum256 not working properpy (or maybe os.ReadFile not working) [x] -> os.ReadFile not working properly when program is writing to that file
// TODO: don't reload when unrelated files get editted [x]
// TODO: auto reload if `back button` get pressed (this does not trigger a GET request from client) [x]
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
    RELOAD_MSG = "RELOAD"
)

var (
    entry_file, root_dir string
    path_seperator string
    websocket_channel, file_cache_channel = make(chan string), make(chan string)
    default_browser_opener string
    has_websocket atomic.Bool
    html_related_files = make([]string, 0, 10) // avoid reload when unrelated to current html file was edited
    html_on_wait_reload = make([]string, 0, 10) // to handle `back button` reloading
)

func main() {
    os_independent_set_args()
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
    fmt.Println("[INFO] Server on:", server.Addr().String())

    // open in browser
    var proc_attr *os.ProcAttr = new(os.ProcAttr)
    _, err = os.StartProcess(default_browser_opener, // start default broser
                            []string{default_browser_opener, fmt.Sprintf("http://%s/", SERVER_ADDR)},
                            proc_attr)
    Check_err(err, true, "Can't start `%s`\n", default_browser_opener)

    go cache.Update_cache_files(file_cache_channel, func(file_path string) {
        for _, v := range html_related_files {
            if file_path == v && has_websocket.Load() {
                websocket_channel <- RELOAD_MSG
                return
            }
        }
        if filepath.Ext(file_path) == ".html" { // html file is edited but not the current displaying one
            html_on_wait_reload = append(html_on_wait_reload, file_path)
        }
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
    fmt.Printf("[INFO] Set default browser open program: %s\n", default_browser_opener)
    fmt.Printf("[INFO] Set path seperator: %s\n", path_seperator)
}

func inject_websocket(file_content []byte) []byte {
    content_str := string(file_content)
    end_html_tag_index := strings.LastIndex(content_str, "</html>")
    if end_html_tag_index == -1 {
        fmt.Println("[ERROR] Write some proper html bro -_-")
        return file_content
    }
    return []byte(content_str[:end_html_tag_index] + WEBSOCKET_INJECT_CODE + content_str[end_html_tag_index:])
}


func handle_websocket(client net.Conn, request *http.HttpRequest) {
    response := ws.Build_websocket_accept(request.Headers["Sec-WebSocket-Key"])
    fmt.Println("[INFO] Successfully connected")
    client.Write(response.Build())
    file_path := root_dir
    if request.File_path == "/" { file_path += path_seperator + entry_file } else { file_path += request.File_path }
    for i, v := range html_on_wait_reload {
        if v == file_path {
            _, err := client.Write(ws.Build_websocket_frame_msg(RELOAD_MSG))
            if !Check_err(err, false, "Can't send message") {
                fmt.Printf("[INFO] Sent message `%s` to client\n", RELOAD_MSG)
            }
            html_on_wait_reload[i] = html_on_wait_reload[len(html_on_wait_reload)-1]
            html_on_wait_reload = html_on_wait_reload[:len(html_on_wait_reload)-1]
            break;
        }
    }
    for {
        msg := <-websocket_channel
        if msg == "CLOSE" {
            _, err := client.Write(ws.CLOSE_FRAME)
            if !Check_err(err, false, "Can't send quit message") {
                fmt.Println("[INFO] Send `quit` message to client")
            }
            break;
        } else if msg == RELOAD_MSG {
            _, err := client.Write(ws.Build_websocket_frame_msg(msg))
            if !Check_err(err, false, "Can't send message") {
                fmt.Printf("[INFO] Sent message `%s` to client\n", RELOAD_MSG)
            }
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
            file_cache_channel <- file_path // add to cache system if it's not already cached
            response = http.BASIC_GET_FILE_RESPONSE
            file_ext := filepath.Ext(file_path)
            response.Headers["Content-type"] = http.CONTENT_TYPES[file_ext]
            response.Headers["Content-Length"] = strconv.Itoa(len(file_content))
            response.Content = file_content
            if file_ext == ".html" {
                html_related_files = html_related_files[:0]
                response.Headers["Content-Length"] = strconv.Itoa(len(file_content) + len(WEBSOCKET_INJECT_CODE))
                response.Content = inject_websocket(file_content)
            }
            fmt.Printf("[INFO] Send file `%s` %d bytes to client\n", file_path, len(file_content))
            html_related_files = append(html_related_files, file_path)
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
