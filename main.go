package main
// TODO: auto reload [x]
//       TODO: learn websocket [x]
//       TODO: inject code into html entry file to run websocket [x]
// TODO: CLEAN UP THIS DUMP MESS T_T [x]
// TODO: maybe figure out why golang/sha256.Sum256 not working properpy (or maybe os.ReadFile not working) [x] -> os.ReadFile not working properly when program is writing to that file
// TODO: don't reload when unrelated files get editted [x]
// TODO: auto reload if `back button` get pressed (this does not trigger a GET request from client) [x]
// TODO: use chunk encoding for large file ? [ ]
import(
    "sync"
    "slices"
    "livesv/Util"
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
    INIT_BUFFER_SIZE = 10
    WEBSOCKET_INJECT_CODE = `
    <script>
        if ('WebSocket' in window) {
            function refreshCSS() {
				var sheets = [].slice.call(document.getElementsByTagName("link"));
				var head = document.getElementsByTagName("head")[0];
				for (var i = 0; i < sheets.length; ++i) {
					var elem = sheets[i];
					var parent = elem.parentElement || head;
					parent.removeChild(elem);
					parent.appendChild(elem);
				}
			}
            let protocol = window.location.protocol === "http:" ? "ws://" : "wss://"
            let address = protocol + window.location.host + window.location.pathname
            let socket = new WebSocket(address)
            socket.onmessage = function(msg) {
                if (msg.data === "RELOAD") {
                    window.location.reload()
                    console.log("RELOADED")
                }
                else if (msg.data === "RELOAD_CSS") {
                    refreshCSS()
                    console.log("CSS RELOADED")
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
    SERVER_ADDR    = "localhost:13123"
    RELOAD_MSG     = "RELOAD"
    RELOAD_CSS_MSG = "RELOAD_CSS"
)

var (
    entry_file, root_dir string
    current_html string // update when ever a html file websocket connect and when a html file is sent to client
    websocket_channel, file_cache_channel = make(chan string), make(chan string)
    default_browser_opener string
    has_websocket atomic.Bool
    files_on_wait_reload = make([]string, 0, 10) // to handle `back button` reloading
    related_files = make(map[string][]string)
    related_files_mutex sync.Mutex
    files_on_wait_mutex sync.Mutex
)

func main() {
    if len(os.Args) <= 1 {
        fmt.Println("USAGE: progname `html file`")
        os.Exit(1)
    }
    if _, err := os.Stat(os.Args[1]); err != nil {
        Util.Check_err(err, true, fmt.Sprintf("`%s` not found.", os.Args[1]))
    }
    root_dir = filepath.Dir(os.Args[1])
    entry_file = filepath.Base(os.Args[1])
    Util.Log(Util.INFO, "Start server with file `%s`", entry_file)
    Util.Log(Util.INFO, "Root directory `%s`", root_dir)

    server, err := net.Listen("tcp", SERVER_ADDR)
    Util.Check_err(err, true, "Can't create server")
    defer server.Close()
    Util.Log(Util.INFO, "Server on: %s", server.Addr().String())

    open_default_browser()

    go cache.Update_cache_files(file_cache_channel, on_file_change)
    for {
        client, err := server.Accept()
        if Util.Check_err(err, false, "Can't accep client") { continue }
        go handle_client(client)
    }

}

func on_file_change(file_path string) {
    if !has_websocket.Load() { return }
    related_files_mutex.Lock()
    if index := slices.Index(related_files[current_html], file_path); index != -1 {
        if filepath.Ext(file_path) == ".css" { websocket_channel <- RELOAD_CSS_MSG } else { websocket_channel <- RELOAD_MSG }
        related_files_mutex.Unlock()
        return
    }
    related_files_mutex.Unlock()
    // file is edited but not the currently in use
    files_on_wait_mutex.Lock()
    if index := slices.Index(files_on_wait_reload, file_path); index == -1 {
        files_on_wait_reload = append(files_on_wait_reload, file_path)
        Util.Log(Util.INFO, "Add `%s` to list of waiting to update files", file_path)
    }
    files_on_wait_mutex.Unlock()
}

func open_default_browser() {
    var attr os.ProcAttr
    switch runtime.GOOS {
    case "windows":
        _, err := os.StartProcess("C:\\Windows\\System32\\cmd.exe", []string{"C:\\Windows\\System32\\cmd.exe", "http://" + SERVER_ADDR}, &attr)
        Util.Check_err(err, false, "Can't start default server", fmt.Sprintf("Open `http://%s` on a browser", SERVER_ADDR))
    case "linux":
        _, err := os.StartProcess("/usr/bin/xdg-open", []string{"/usr/bin/xdg-open", "http://" + SERVER_ADDR}, &attr)
        Util.Check_err(err, false, "Can't start default server", fmt.Sprintf("Open `http://%s` on a browser", SERVER_ADDR))
    default:
        Util.Log(Util.WARN, "Unknown platform, the program may not work correctly")
        Util.Log(Util.INFO, "\tOpen `http://%s` on a browser", SERVER_ADDR)
    }
}

func inject_websocket(file_path string, file_content []byte) []byte {
    content_str := string(file_content)
    end_html_tag_index := strings.LastIndex(content_str, "</html>")
    if end_html_tag_index == -1 {
        Util.Log(Util.WARN, "Can't inject websocket into `%s`", file_path)
        Util.Log(Util.INFO, "\tCan't find end tag </html>.")
        return file_content
    }
    return []byte(content_str[:end_html_tag_index] + WEBSOCKET_INJECT_CODE + content_str[end_html_tag_index:])
}

func handle_websocket(client net.Conn, request *http.HttpRequest) {
    response := ws.Build_websocket_accept(request.Headers["Sec-WebSocket-Key"])
    Util.Log(Util.INFO, "Successfully connected")
    client.Write(response.Build())
    file_path := root_dir
    if request.Url.Path == "/" { file_path += "/" + entry_file } else { file_path += request.Url.Path }
    current_html = file_path // maybe unecessary but just do it anyway (this could have already happen when send html file)

    related_files_mutex.Lock()
    for _, f := range related_files[current_html] {
        files_on_wait_mutex.Lock()
        if i := slices.Index(files_on_wait_reload, f); i != -1 {
            _, err := client.Write(ws.Build_websocket_frame_msg(RELOAD_MSG))
            if !Util.Check_err(err, false, "Can't send message") {
                Util.Log(Util.INFO, "Sent message `%s` to client", RELOAD_MSG)
            }
            files_on_wait_reload[i] = files_on_wait_reload[len(files_on_wait_reload)-1]
            files_on_wait_reload = files_on_wait_reload[:len(files_on_wait_reload)-1]
        }
        files_on_wait_mutex.Unlock()
    }
    related_files_mutex.Unlock()

    for {
        msg := <-websocket_channel
        if msg == "CLOSE" {
            _, err := client.Write(ws.CLOSE_FRAME)
            if !Util.Check_err(err, false, "Can't send quit message") {
                Util.Log(Util.INFO, "Send `quit` message to client")
            }
            break;
        } else {
            _, err := client.Write(ws.Build_websocket_frame_msg(msg))
            if !Util.Check_err(err, false, "Can't send message") {
                Util.Log(Util.INFO, "Sent message `%s` to client", msg)
            }
        }
    }
}

func handle_http(client net.Conn, request *http.HttpRequest) {
    switch request.Method {
    case "GET":
        var response http.HttpResponse
        file_path := root_dir
        if request.Url.Path == "/" { file_path += "/" + entry_file } else { file_path += request.Url.Path }
        file_content := Util.Os_independent_readfile(file_path)
        if file_content != nil {
            file_cache_channel <- file_path // add to cache system if it's not already cached
            http.Make_basic_ok(&response)
            file_ext := filepath.Ext(file_path)
            response.Headers["Content-type"] = http.CONTENT_TYPES[file_ext]
            response.Headers["Content-Length"] = strconv.Itoa(len(file_content))
            response.Content = file_content
            if file_ext == ".html" {
                related_files_mutex.Lock()
                if _, found := related_files[file_path]; !found {
                    related_files[file_path] = make([]string, 0, INIT_BUFFER_SIZE)
                } else {
                    related_files[file_path] = related_files[file_path][:0]
                }
                related_files_mutex.Unlock()
                response.Headers["Content-Length"] = strconv.Itoa(len(file_content) + len(WEBSOCKET_INJECT_CODE))
                response.Content                   = inject_websocket(file_path, file_content)
                current_html                       = file_path // maybe unecessary but just do it anyway (this could have already happen when handle socket)
            }
            Util.Log(Util.INFO, "Send file `%s` %d bytes to client", file_path, len(file_content))

            // add file path to html related files if necessary
            related_files_mutex.Lock()
            temp := related_files[current_html]
            if index := slices.Index(temp, file_path); index == -1 {
                related_files[current_html] = append(temp, file_path)
            }
            related_files_mutex.Unlock()
        } else {
            http.Make_file_not_found(&response, file_path)
        }
        client.Write(response.Build())
    default:
        Util.Unimplemented()
    }
}

func handle_client(client net.Conn) {
    defer client.Close()
    buffer := make([]byte, 1024)
    n, err := client.Read(buffer)
    if Util.Check_err(err, false, "Can't read from client") { return }
    request := http.Parse_request(string(buffer[:n]))
    if (request == nil) { return }
    if request.Headers["Connection"] == "Upgrade" && request.Headers["Upgrade"] == "websocket" {
        if has_websocket.Load() {
            websocket_channel <- "CLOSE"
            for has_websocket.Load() {}
        }
        has_websocket.Store(true)
        handle_websocket(client, request)
        has_websocket.Store(false)
    } else {
        handle_http(client, request)
    }
    Util.Log(Util.INFO, "Client closed")
}
