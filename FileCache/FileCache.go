// cache modification time and sha256 checksum to determine when a file need to be reloaded
package FileCache

import (
    "strings"
    "time"
    "fmt"
    "os"
    "os/exec"
    "crypto/sha256"
)

type FileCacheEntry struct {
    last_modified time.Time
    last_hash string
}
type FilePath = string
type FileCache = map[FilePath]FileCacheEntry

var (
    files_cache = make(FileCache)
    use_sha256_pack = false
)

func Get_last_modified(file_path string) *time.Time {
    file, err := os.Open(file_path)
    if err != nil { return nil }
    defer file.Close()
    info, err := file.Stat()
    if err != nil {
        fmt.Println("[WARNING] Can't stat file", file_path)
        return nil
    }
    result := new(time.Time)
    *result = info.ModTime()
    return result
}

func Get_file_content(file_path string) []byte {
    content, err := os.ReadFile(file_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] Can't read file %s\n", file_path)
        return nil
    }
    return content
}

func Get_sha256(file_path string) string {
    var out []byte
    var err error
    if !use_sha256_pack {
        out, err = exec.Command("sha256sum", file_path).Output()
        if err != nil {
            fmt.Println("[ERROR] No command sha256sum found, switch to dump version sha256 package")
            use_sha256_pack = true
        }
    } else {
        temp := sha256.Sum256(Get_file_content(file_path))
        out = temp[:]
    }
    return string(out)
}

func Is_cached(file_path string) bool {
    if _, found := files_cache[file_path]; found {
        return true
    }
    return false
}

func Update_cache_files(ch chan string, on_file_change func(file_path string)) {
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
        for file_path, entry := range files_cache {
            last_modified := Get_last_modified(file_path)
            if last_modified != nil && !entry.last_modified.Equal(*last_modified) {
                out_str := Get_sha256(file_path)
                new_hash := out_str[:strings.Index(out_str, "  ")]
                if new_hash != entry.last_hash {
                    if entry.last_hash != "" { on_file_change(file_path) }
                    fmt.Printf("[INFO] Updated sha512 for file %s\n", file_path)
                }
                files_cache[file_path] = FileCacheEntry{
                    last_modified: *last_modified,
                    last_hash: new_hash,
                }
            }
        }
    }
}
