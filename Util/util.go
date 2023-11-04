package Util
import (
    "fmt"
    "log"
    "os"
    "path/filepath"
)
const (
   INFO = "[INFO]"
   WARN = "[WARNING]"
   ERR  = "[ERROR]"
)
func Log(log_type string, format string, args ...any) {
    log.Printf("%-10s %s", log_type, fmt.Sprintf(format, args...))
}
func Unimplemented() {
    Log(ERR, "Umimplemented\n")
    os.Exit(1)
}
func Check_err(err error, fatal bool, info ...string) bool {
    if err != nil {
        if fatal { Log(ERR, err.Error()); } else { Log(WARN, err.Error()); }
        for _, v := range info { Log(INFO, "    %s", v); }
        if fatal { os.Exit(1) }
        return true;
    }
    return false;
}
func Os_independent_readfile(file_path string) []byte {
    file_path = filepath.FromSlash(file_path)
    content, err := os.ReadFile(file_path)
    if Check_err(err, false, "Can't read file " + file_path) { return nil }
    return content
}
