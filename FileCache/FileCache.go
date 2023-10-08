// cache modification time and sha256 checksum to determine when a file need to be reloaded
package FileCache

import (
    "encoding/hex"
    "time"
    "fmt"
    "os"
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
    cannot_read_tolerance := 100 // for cases like when a program is writing to that file
    count := 0
    content, err := os.ReadFile(file_path)
    for count < cannot_read_tolerance && err != nil && os.IsNotExist(err) || len(content) == 0 {
        content, err = os.ReadFile(file_path)
    }
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] %s\n\t[INFO] %s %s\n", err.Error(), "Can't read file", file_path)
        return nil
    }
    return content
}

func Get_sha256(file_path string) string {
    sum := sha256.Sum256(Get_file_content(file_path))
    return hex.EncodeToString(sum[:])
}

func Update_cache_files(ch chan string, on_file_change func(string)) {
    for {
        select {
        case file_path, ok := <-ch:
            if ok {
                if _, found := files_cache[file_path]; !found {
                    files_cache[file_path] = FileCacheEntry{} 
                    fmt.Printf("[INFO] Cache file `%s`\n", file_path)
                }
            } else {
                fmt.Println("[INFO] Channel closed!")
            }
        default:
        }
        for file_path, entry := range files_cache {
            last_modified := Get_last_modified(file_path)
            if last_modified != nil && !entry.last_modified.Equal(*last_modified) {
                new_hash := Get_sha256(file_path)
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
        time.Sleep(100)
    }
}
