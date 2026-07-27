package luahost

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MaterializeEmbeddedLua copies the embedded catalog tree into
// $UserCacheDir/gdbforge/embedded-lua/ so LoadScriptFile and lua_dir()
// sidecars (e.g. r5_target.xml) use real filesystem paths.
// Existing cache is replaced each call (catalog is small).
func MaterializeEmbeddedLua(fsys fs.FS) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("nil embedded fs")
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(cacheRoot, "gdbforge", "embedded-lua")
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}
