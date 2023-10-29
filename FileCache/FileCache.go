// cache modification time and sha256 checksum to determine when a file need to be reloaded
package FileCache

import (
    "livesv/Util"
    "encoding/hex"
    "time"
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
    on_waiting_removed = make([]string, 0, 10)
)

func Get_sha256(file_path string) string {
    sum := sha256.Sum256(Util.Os_independent_readfile(file_path))
    return hex.EncodeToString(sum[:])
}

func Update_cache_files(ch chan string, on_file_change func(string)) {
    for {
        select {
        case file_path, ok := <-ch:
            if ok {
                if _, found := files_cache[file_path]; !found {
                    files_cache[file_path] = FileCacheEntry{}
                    Util.Log(Util.INFO, "Cache file `%s`", file_path)
                }
            } else {
                Util.Log(Util.INFO, "Channel closed!")
            }
        default:
            time.Sleep(100 * time.Millisecond)
        }
        for file_path, entry := range files_cache {
            info, err := os.Stat(file_path)
            if Util.Check_err(err, false, "Can't stat file") {
                on_waiting_removed = append(on_waiting_removed, file_path)
            } else if !entry.last_modified.Equal(info.ModTime()) {
                if entry.last_hash != "" { time.Sleep(100 * time.Millisecond) }// wait a for the file to be saved completely
                new_hash := Get_sha256(file_path)
                if new_hash != entry.last_hash {
                    if entry.last_hash != "" { on_file_change(file_path) }
                    Util.Log(Util.INFO, "Updated sha512 for file %s", file_path)
                }
                files_cache[file_path] = FileCacheEntry{
                    last_modified: info.ModTime(),
                    last_hash: new_hash,
                }
            }
        }
        for _, f := range on_waiting_removed { delete(files_cache, f) }
        on_waiting_removed = on_waiting_removed[:0]
    }
}
