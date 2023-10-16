package Util
import (
    "fmt"
    "os"
    "strings"
    "path/filepath"
)
func Check_err(err error, fatal bool, info ...string) bool {
    if err != nil {
        var msg_builder strings.Builder
        if fatal { msg_builder.WriteString("[ERROR] ") } else { msg_builder.WriteString("[WARNING] ") }
        msg_builder.WriteString(err.Error())
        msg_builder.WriteString("\n")
        for _, v := range info {
            msg_builder.WriteString("\t [INFO] ")
            msg_builder.WriteString(v)
            msg_builder.WriteString("\n")
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
func Os_independent_readfile(file_path string) []byte {
    file_path = filepath.FromSlash(file_path)
    content, err := os.ReadFile(file_path)
    if Check_err(err, false, "Can't read file " + file_path) { return nil }
    return content
}

